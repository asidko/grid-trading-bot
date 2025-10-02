package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/grid-trading-bot/services/order-assurance/internal/api"
	"github.com/grid-trading-bot/services/order-assurance/internal/config"
	"github.com/grid-trading-bot/services/order-assurance/internal/exchange"
	"github.com/grid-trading-bot/services/order-assurance/internal/service"
	"github.com/grid-trading-bot/services/order-assurance/internal/client"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found: %v", err)
	}

	// Load configuration
	cfg := config.LoadConfig()

	// Log whether we have credentials
	if cfg.BinanceAPIKey == "" || cfg.BinanceSecret == "" {
		log.Println("WARNING: Binance API credentials not configured - order placement will fail")
	} else {
		log.Println("Binance API credentials configured")
	}

	// Create Binance client (works with or without credentials)
	binanceClient := exchange.NewBinanceClient(
		cfg.BinanceAPIKey,
		cfg.BinanceSecret,
	)

	// Create grid-trading client notifier
	gridClient := client.NewNotifier(cfg.GridTradingURL)

	// Create order service
	orderService := service.NewOrderService(binanceClient, gridClient)

	// Create API handlers
	handlers := api.NewHandlers(orderService)

	// Setup routes
	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}


	// Start server
	go func() {
		log.Printf("Order Assurance Service starting on port %s", cfg.ServerPort)
		log.Println("Using Binance Production API")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed:", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Shutdown server
	if err := srv.Close(); err != nil {
		log.Printf("Server close error: %v", err)
	}

	fmt.Println("Server stopped")
}