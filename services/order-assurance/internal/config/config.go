package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort     string
	BinanceAPIKey  string
	BinanceSecret  string
	BinanceTestnet bool
	GridTradingURL string
}

func LoadConfig() *Config {
	testnet := false
	if t := os.Getenv("BINANCE_TESTNET"); t != "" {
		if v, _ := strconv.ParseBool(t); v {
			testnet = true
		}
	}

	return &Config{
		ServerPort:     getEnv("SERVER_PORT", "9090"),
		BinanceAPIKey:  getEnv("BINANCE_API_KEY", ""),
		BinanceSecret:  getEnv("BINANCE_API_SECRET", ""),
		BinanceTestnet: testnet,
		GridTradingURL: getEnv("GRID_TRADING_URL", "http://localhost:8080"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}