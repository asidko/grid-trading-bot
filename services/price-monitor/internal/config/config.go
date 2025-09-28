package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ServerPort        string
	GridTradingURL    string
	MonitoredSymbols  []string
	TriggerIntervalMs int
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

	monitoredSymbols := os.Getenv("MONITORED_SYMBOLS")
	if monitoredSymbols == "" {
		log.Fatal("MONITORED_SYMBOLS is required")
	}

	triggerIntervalStr := os.Getenv("TRIGGER_INTERVAL_MS")
	if triggerIntervalStr == "" {
		log.Fatal("TRIGGER_INTERVAL_MS is required")
	}

	symbols := strings.Split(monitoredSymbols, ",")
	for i := range symbols {
		symbols[i] = strings.TrimSpace(symbols[i])
		if symbols[i] == "" {
			log.Fatal("Empty symbol in MONITORED_SYMBOLS")
		}
	}

	triggerInterval, err := strconv.Atoi(triggerIntervalStr)
	if err != nil || triggerInterval <= 0 {
		log.Fatal("TRIGGER_INTERVAL_MS must be a positive integer")
	}

	return &Config{
		ServerPort:        serverPort,
		GridTradingURL:    gridTradingURL,
		MonitoredSymbols:  symbols,
		TriggerIntervalMs: triggerInterval,
	}
}