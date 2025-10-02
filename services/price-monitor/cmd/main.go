package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/grid-trading-bot/services/price-monitor/internal/client"
	"github.com/grid-trading-bot/services/price-monitor/internal/config"
	"github.com/grid-trading-bot/services/price-monitor/internal/ticker"
	"github.com/shopspring/decimal"
)

type PriceMonitor struct {
	cfg         *config.Config
	ticker      *ticker.BinanceTicker
	gridClient  *client.GridTradingClient
	lastTrigger map[string]time.Time
	lastPrice   map[string]decimal.Decimal
	symbols     []string
	mu          sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	lastCheckTime    time.Time
	lastSymbolsFetch time.Time
	checkCount       int64
	errorCount       int64
}

func NewPriceMonitor(cfg *config.Config) *PriceMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &PriceMonitor{
		cfg:         cfg,
		ticker:      ticker.NewBinanceTicker(),
		gridClient:  client.NewGridTradingClient(cfg.GridTradingURL),
		lastTrigger: make(map[string]time.Time),
		lastPrice:   make(map[string]decimal.Decimal),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (pm *PriceMonitor) Start() error {
	// Fetch symbols from grid service
	if err := pm.refreshSymbols(); err != nil {
		log.Printf("Warning: Failed to fetch symbols from grid service: %v", err)
		log.Printf("Will retry in next cycle")
	}

	log.Printf("Starting price monitor with polling interval: %dms", pm.cfg.PriceCheckIntervalMs)
	log.Printf("Min price change for trigger: %.4f%%", pm.cfg.MinPriceChangePct)

	// Start the polling loop
	pm.wg.Add(1)
	go pm.pollingLoop()

	return nil
}

func (pm *PriceMonitor) refreshSymbols() error {
	symbols, err := pm.gridClient.GetGridSymbols()
	if err != nil {
		return err
	}

	pm.mu.Lock()
	pm.symbols = symbols
	pm.lastSymbolsFetch = time.Now()
	pm.mu.Unlock()

	return nil
}

func (pm *PriceMonitor) pollingLoop() {
	defer pm.wg.Done()

	checkInterval := time.Duration(pm.cfg.PriceCheckIntervalMs) * time.Millisecond
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Do initial check immediately
	pm.checkPrices()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			// Refresh symbols every other run (on even check counts)
			pm.mu.RLock()
			shouldRefresh := pm.checkCount%2 == 0
			pm.mu.RUnlock()

			if shouldRefresh {
				if err := pm.refreshSymbols(); err != nil {
					log.Printf("Failed to refresh symbols: %v", err)
				}
			}
			pm.checkPrices()
		}
	}
}

func (pm *PriceMonitor) checkPrices() {
	pm.mu.Lock()
	pm.lastCheckTime = time.Now()
	pm.checkCount++
	symbols := pm.symbols
	pm.mu.Unlock()

	// Skip if no symbols to monitor
	if len(symbols) == 0 {
		return
	}

	// Fetch prices for all symbols
	prices, err := pm.ticker.GetPrices(symbols)
	if err != nil {
		pm.mu.Lock()
		pm.errorCount++
		pm.mu.Unlock()
		log.Printf("Failed to fetch prices: %v", err)
		return
	}

	// Process each price update
	for symbol, price := range prices {
		pm.handlePriceUpdate(symbol, price)
	}
}

func (pm *PriceMonitor) handlePriceUpdate(symbol string, price decimal.Decimal) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if we should throttle this update
	if lastTime, ok := pm.lastTrigger[symbol]; ok {
		if time.Since(lastTime) < time.Duration(pm.cfg.TriggerIntervalMs)*time.Millisecond {
			return // Skip - too soon
		}
	}

	// Check if price changed significantly
	if lastPrice, ok := pm.lastPrice[symbol]; ok {
		change := price.Sub(lastPrice).Abs().Div(lastPrice).Mul(decimal.NewFromInt(100))
		if change.LessThan(decimal.NewFromFloat(pm.cfg.MinPriceChangePct)) {
			return // Skip - insignificant change
		}
	}

	// Send trigger to grid-trading
	if err := pm.gridClient.SendPriceTrigger(symbol, price); err != nil {
		log.Printf("Failed to send trigger for %s at %s: %v",
			symbol, price, err)
		return
	}

	// Update tracking
	pm.lastTrigger[symbol] = time.Now()
	pm.lastPrice[symbol] = price

	log.Printf("Triggered %s at %s", symbol, price)
}

func (pm *PriceMonitor) GetStatus() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status := make(map[string]interface{})
	status["monitoring"] = true
	status["monitored_symbols"] = pm.symbols
	status["last_symbols_fetch"] = pm.lastSymbolsFetch.Format(time.RFC3339)
	status["price_check_interval_ms"] = pm.cfg.PriceCheckIntervalMs
	status["check_count"] = pm.checkCount
	status["error_count"] = pm.errorCount
	status["last_check_time"] = pm.lastCheckTime.Format(time.RFC3339)

	lastPrices := make(map[string]string)
	for symbol, price := range pm.lastPrice {
		lastPrices[symbol] = price.String()
	}
	status["last_prices"] = lastPrices

	lastTriggers := make(map[string]string)
	for symbol, t := range pm.lastTrigger {
		lastTriggers[symbol] = t.Format(time.RFC3339)
	}
	status["last_triggers"] = lastTriggers

	return status
}

func (pm *PriceMonitor) Shutdown() {
	log.Println("Shutting down price monitor...")
	pm.cancel()
	pm.wg.Wait()
}

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Create price monitor
	monitor := NewPriceMonitor(cfg)

	// Start monitoring
	if err := monitor.Start(); err != nil {
		log.Fatal("Failed to start monitor:", err)
	}

	// Setup HTTP server for health checks
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	// Status endpoint
	router.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(monitor.GetStatus())
	})

	// Start HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Printf("Price Monitor starting on port %s", cfg.ServerPort)
		log.Printf("Using Binance REST API with polling")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed:", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	monitor.Shutdown()
	srv.Shutdown(ctx)
	log.Println("Server stopped")
}