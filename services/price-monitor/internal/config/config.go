package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	ServerPort           string
	GridTradingURL       string
	PriceCheckIntervalMs int
	MinPriceChangePct    float64
}

func LoadConfig() *Config {
	// Required environment variables
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		log.Fatal("SERVER_PORT is required")
	}

	gridTradingURL := os.Getenv("GRID_TRADING_URL")
	if gridTradingURL == "" {
		log.Fatal("GRID_TRADING_URL is required")
	}

	priceCheckIntervalStr := os.Getenv("PRICE_CHECK_INTERVAL_MS")
	if priceCheckIntervalStr == "" {
		priceCheckIntervalStr = "10000" // Default to 10 seconds
	}

	minPriceChangeStr := os.Getenv("MIN_PRICE_CHANGE_PCT")
	if minPriceChangeStr == "" {
		minPriceChangeStr = "0.01" // Default to 0.01%
	}

	priceCheckInterval, err := strconv.Atoi(priceCheckIntervalStr)
	if err != nil || priceCheckInterval <= 0 {
		log.Fatal("PRICE_CHECK_INTERVAL_MS must be a positive integer")
	}

	minPriceChange, err := strconv.ParseFloat(minPriceChangeStr, 64)
	if err != nil || minPriceChange < 0 {
		log.Fatal("MIN_PRICE_CHANGE_PCT must be a non-negative number")
	}

	return &Config{
		ServerPort:           serverPort,
		GridTradingURL:       gridTradingURL,
		PriceCheckIntervalMs: priceCheckInterval,
		MinPriceChangePct:    minPriceChange,
	}
}