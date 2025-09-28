package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/grid-trading-bot/services/price-monitor/internal/client"
	"github.com/grid-trading-bot/services/price-monitor/internal/config"
	"github.com/grid-trading-bot/services/price-monitor/internal/websocket"
	"github.com/shopspring/decimal"
)

type PriceMonitor struct {
	cfg           *config.Config
	ws            *websocket.BinanceWS
	gridClient    *client.GridTradingClient
	lastTrigger   map[string]time.Time
	lastPrice     map[string]decimal.Decimal
	mu            sync.RWMutex

	ctx           context.Context
	cancel        context.CancelFunc
	connected     atomic.Bool
	wg            sync.WaitGroup
}

func NewPriceMonitor(cfg *config.Config) *PriceMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &PriceMonitor{
		cfg:         cfg,
		gridClient:  client.NewGridTradingClient(cfg.GridTradingURL),
		lastTrigger: make(map[string]time.Time),
		lastPrice:   make(map[string]decimal.Decimal),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (pm *PriceMonitor) Start() error {
	// Validate configuration
	if len(pm.cfg.MonitoredSymbols) == 0 {
		log.Fatal("No symbols configured for monitoring")
	}

	// Start the connection manager
	pm.wg.Add(1)
	go pm.connectionManager()

	return nil
}

func (pm *PriceMonitor) connectionManager() {
	defer pm.wg.Done()

	reconnectDelay := 1 * time.Second
	maxReconnectDelay := 60 * time.Second

	for {
		select {
		case <-pm.ctx.Done():
			return
		default:
		}

		// Create and connect
		pm.ws = websocket.NewBinanceWS(pm.cfg.MonitoredSymbols)
		if err := pm.ws.Connect(); err != nil {
			log.Printf("Connection failed: %v, retrying in %v", err, reconnectDelay)
			time.Sleep(reconnectDelay)

			// Exponential backoff
			reconnectDelay *= 2
			if reconnectDelay > maxReconnectDelay {
				reconnectDelay = maxReconnectDelay
			}
			continue
		}

		pm.connected.Store(true)
		reconnectDelay = 1 * time.Second // Reset delay on successful connection
		log.Printf("Connected to Binance WebSocket for symbols: %v", pm.cfg.MonitoredSymbols)

		// Process updates until error
		pm.processUpdates()

		pm.connected.Store(false)

		// Clean up old connection
		if pm.ws != nil {
			pm.ws.Close()
			pm.ws = nil
		}

		// Check if we should exit
		select {
		case <-pm.ctx.Done():
			return
		case <-time.After(reconnectDelay):
			// Continue to reconnect
		}
	}
}

func (pm *PriceMonitor) processUpdates() {
	for {
		select {
		case <-pm.ctx.Done():
			return

		case update, ok := <-pm.ws.PriceChannel():
			if !ok {
				return // Channel closed, need to reconnect
			}
			pm.handlePriceUpdate(update)

		case err, ok := <-pm.ws.ErrorChannel():
			if !ok {
				return // Channel closed
			}
			log.Printf("WebSocket error: %v", err)
			return // Exit to trigger reconnection
		}
	}
}

func (pm *PriceMonitor) handlePriceUpdate(update websocket.PriceUpdate) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if we should throttle this update
	if lastTime, ok := pm.lastTrigger[update.Symbol]; ok {
		if time.Since(lastTime) < time.Duration(pm.cfg.TriggerIntervalMs)*time.Millisecond {
			return // Skip - too soon
		}
	}

	// Check if price changed significantly (0.01% change)
	if lastPrice, ok := pm.lastPrice[update.Symbol]; ok {
		change := update.Price.Sub(lastPrice).Abs().Div(lastPrice)
		if change.LessThan(decimal.NewFromFloat(0.0001)) {
			return // Skip - insignificant change
		}
	}

	// Send trigger to grid-trading
	if err := pm.gridClient.SendPriceTrigger(update.Symbol, update.Price); err != nil {
		log.Printf("Failed to send trigger for %s at %s: %v",
			update.Symbol, update.Price, err)
		return
	}

	// Update tracking
	pm.lastTrigger[update.Symbol] = time.Now()
	pm.lastPrice[update.Symbol] = update.Price

	log.Printf("Triggered %s at %s", update.Symbol, update.Price)
}

func (pm *PriceMonitor) GetStatus() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status := make(map[string]interface{})
	status["connected"] = pm.connected.Load()
	status["monitored_symbols"] = pm.cfg.MonitoredSymbols

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

	if pm.ws != nil {
		pm.ws.Close()
	}
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
		status := "healthy"
		if !monitor.connected.Load() {
			status = "reconnecting"
		}
		json.NewEncoder(w).Encode(map[string]string{"status": status})
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
		log.Printf("Monitoring symbols: %v", cfg.MonitoredSymbols)
		log.Println("Using Binance Production WebSocket")

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