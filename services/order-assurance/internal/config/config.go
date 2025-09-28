package config

import (
	"os"
)

type Config struct {
	ServerPort     string
	BinanceAPIKey  string
	BinanceSecret  string
	GridTradingURL string
}

func LoadConfig() *Config {
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "9090" // Only default kept for local dev
	}

	apiKey := os.Getenv("BINANCE_API_KEY")
	apiSecret := os.Getenv("BINANCE_API_SECRET")

	gridTradingURL := os.Getenv("GRID_TRADING_URL")
	if gridTradingURL == "" {
		gridTradingURL = "http://localhost:8080" // Only default kept for local dev
	}

	return &Config{
		ServerPort:     serverPort,
		BinanceAPIKey:  apiKey,
		BinanceSecret:  apiSecret,
		GridTradingURL: gridTradingURL,
	}
}