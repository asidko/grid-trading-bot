package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort        string
	DBPath            string
	OrderAssuranceURL string
	SyncJobEnabled    bool
	SyncJobCron       string
	TradingFee        float64
}

func LoadConfig() *Config {
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./grid_trading.db"
	}

	orderAssuranceURL := os.Getenv("ORDER_ASSURANCE_URL")
	if orderAssuranceURL == "" {
		orderAssuranceURL = "http://localhost:9090"
	}

	syncEnabled, _ := strconv.ParseBool(os.Getenv("SYNC_JOB_ENABLED"))

	syncCron := os.Getenv("SYNC_JOB_CRON")
	if syncCron == "" {
		syncCron = "0 * * * *"
	}

	tradingFeeStr := os.Getenv("TRADING_FEE")
	tradingFee := 0.1
	if tradingFeeStr != "" {
		if parsed, err := strconv.ParseFloat(tradingFeeStr, 64); err == nil {
			tradingFee = parsed
		}
	}

	return &Config{
		ServerPort:        serverPort,
		DBPath:            dbPath,
		OrderAssuranceURL: orderAssuranceURL,
		SyncJobEnabled:    syncEnabled,
		SyncJobCron:       syncCron,
		TradingFee:        tradingFee,
	}
}