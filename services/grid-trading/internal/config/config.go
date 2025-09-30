package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort           string
	DBHost               string
	DBPort               int
	DBUser               string
	DBPassword           string
	DBName               string
	DBSSLMode            string
	OrderAssuranceURL    string
	SyncJobEnabled       bool
	SyncJobCron          string
	TradingFee           float64
}

func LoadConfig() *Config {
	// Required variables
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080" // Only default kept for local dev
	}

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost" // Only default kept for local dev
	}

	dbPortStr := os.Getenv("DB_PORT")
	if dbPortStr == "" {
		dbPortStr = "5432" // Only default kept for local dev
	}
	dbPort, err := strconv.Atoi(dbPortStr)
	if err != nil {
		dbPort = 5432
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres" // Only default kept for local dev
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres" // Only default kept for local dev
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "grid_trading" // Only default kept for local dev
	}

	dbSSLMode := os.Getenv("DB_SSL_MODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable" // Only default kept for local dev
	}

	orderAssuranceURL := os.Getenv("ORDER_ASSURANCE_URL")
	if orderAssuranceURL == "" {
		orderAssuranceURL = "http://localhost:9090" // Only default kept for local dev
	}

	syncEnabled, _ := strconv.ParseBool(os.Getenv("SYNC_JOB_ENABLED"))

	syncCron := os.Getenv("SYNC_JOB_CRON")
	if syncCron == "" {
		syncCron = "0 * * * *" // Hourly default
	}

	tradingFeeStr := os.Getenv("TRADING_FEE")
	tradingFee := 0.1 // Binance spot default: 0.1%
	if tradingFeeStr != "" {
		if parsed, err := strconv.ParseFloat(tradingFeeStr, 64); err == nil {
			tradingFee = parsed
		}
	}

	return &Config{
		ServerPort:        serverPort,
		DBHost:            dbHost,
		DBPort:            dbPort,
		DBUser:            dbUser,
		DBPassword:        dbPassword,
		DBName:            dbName,
		DBSSLMode:         dbSSLMode,
		OrderAssuranceURL: orderAssuranceURL,
		SyncJobEnabled:    syncEnabled,
		SyncJobCron:       syncCron,
		TradingFee:        tradingFee,
	}
}