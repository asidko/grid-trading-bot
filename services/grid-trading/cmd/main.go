package main

import (
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
)

func main() {

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
	gridService := service.NewGridService(repo, txRepo, assuranceClient, cfg.TradingFee)

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