package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/grid-trading-bot/services/grid-trading/internal/api"
	"github.com/grid-trading-bot/services/grid-trading/internal/client"
	"github.com/grid-trading-bot/services/grid-trading/internal/config"
	"github.com/grid-trading-bot/services/grid-trading/internal/database"
	"github.com/grid-trading-bot/services/grid-trading/internal/repository"
	"github.com/grid-trading-bot/services/grid-trading/internal/service"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/shopspring/decimal"
)

func main() {
	var (
		initGrid = flag.Bool("init-grid", false, "Initialize grid levels")
		symbol   = flag.String("symbol", "", "Trading symbol (e.g., ETH, BTC)")
		minPrice = flag.String("min-price", "", "Minimum price for grid")
		maxPrice = flag.String("max-price", "", "Maximum price for grid")
		gridStep = flag.String("grid-step", "", "Price step between levels")
		buyAmount = flag.String("buy-amount", "", "USDT amount per level")
	)
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found: %v", err)
	}

	cfg := config.LoadConfig()

	dbCfg := database.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	}

	db, err := database.NewConnection(dbCfg)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Run migrations
	migrations := []string{
		"services/grid-trading/migrations/001_create_grid_levels.sql",
		"services/grid-trading/migrations/002_create_transactions.sql",
	}

	for _, migrationFile := range migrations {
		migrationSQL, err := os.ReadFile(migrationFile)
		if err != nil {
			log.Fatalf("Failed to read migration file %s: %v", migrationFile, err)
		}

		if err := database.RunMigrations(db, string(migrationSQL)); err != nil {
			log.Fatalf("Failed to run migration %s: %v", migrationFile, err)
		}
	}

	repo := repository.NewGridLevelRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	assuranceClient := client.NewOrderAssuranceClient(cfg.OrderAssuranceURL)
	gridService := service.NewGridService(repo, txRepo, assuranceClient)

	if *initGrid {
		if *symbol == "" || *minPrice == "" || *maxPrice == "" || *gridStep == "" || *buyAmount == "" {
			log.Fatal("All parameters required for grid initialization: -symbol, -min-price, -max-price, -grid-step, -buy-amount")
		}

		minPriceDec, err := decimal.NewFromString(*minPrice)
		if err != nil {
			log.Fatal("Invalid min price:", err)
		}

		maxPriceDec, err := decimal.NewFromString(*maxPrice)
		if err != nil {
			log.Fatal("Invalid max price:", err)
		}

		gridStepDec, err := decimal.NewFromString(*gridStep)
		if err != nil {
			log.Fatal("Invalid grid step:", err)
		}

		buyAmountDec, err := decimal.NewFromString(*buyAmount)
		if err != nil {
			log.Fatal("Invalid buy amount:", err)
		}

		log.Printf("Initializing grid for %s: range %s-%s, step %s, amount %s USDT",
			*symbol, *minPrice, *maxPrice, *gridStep, *buyAmount)

		if err := gridService.InitializeGrid(*symbol, minPriceDec, maxPriceDec, gridStepDec, buyAmountDec); err != nil {
			log.Fatal("Failed to initialize grid:", err)
		}

		log.Println("Grid initialization complete")
		return
	}

	if cfg.SyncJobEnabled {
		c := cron.New()
		_, err := c.AddFunc(cfg.SyncJobCron, func() {
			log.Println("Running sync job...")
			if err := gridService.SyncOrders(); err != nil {
				log.Printf("Sync job failed: %v", err)
			} else {
				log.Println("Sync job completed")
			}
		})
		if err != nil {
			log.Fatal("Failed to add cron job:", err)
		}
		c.Start()
		defer c.Stop()
		log.Printf("Sync job scheduled with cron: %s", cfg.SyncJobCron)
	}

	handlers := api.NewHandlers(gridService)
	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Printf("Starting server on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed:", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	fmt.Println("Server stopped")
}