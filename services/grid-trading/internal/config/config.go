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
}

func LoadConfig() *Config {
	port := 5432
	if p := os.Getenv("DB_PORT"); p != "" {
		if v, _ := strconv.Atoi(p); v > 0 {
			port = v
		}
	}

	syncEnabled := true
	if s := os.Getenv("SYNC_JOB_ENABLED"); s != "" {
		if v, _ := strconv.ParseBool(s); !v {
			syncEnabled = false
		}
	}

	return &Config{
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		DBHost:            getEnv("DB_HOST", "localhost"),
		DBPort:            port,
		DBUser:            getEnv("DB_USER", "postgres"),
		DBPassword:        getEnv("DB_PASSWORD", "postgres"),
		DBName:            getEnv("DB_NAME", "grid_trading"),
		DBSSLMode:         getEnv("DB_SSL_MODE", "disable"),
		OrderAssuranceURL: getEnv("ORDER_ASSURANCE_URL", "http://localhost:9090"),
		SyncJobEnabled:    syncEnabled,
		SyncJobCron:       getEnv("SYNC_JOB_CRON", "0 * * * *"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}