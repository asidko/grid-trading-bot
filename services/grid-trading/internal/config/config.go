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
	cfg := &Config{
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		DBHost:            getEnv("DB_HOST", "localhost"),
		DBPort:            getEnvAsInt("DB_PORT", 5432),
		DBUser:            getEnv("DB_USER", "postgres"),
		DBPassword:        getEnv("DB_PASSWORD", "postgres"),
		DBName:            getEnv("DB_NAME", "grid_trading"),
		DBSSLMode:         getEnv("DB_SSL_MODE", "disable"),
		OrderAssuranceURL: getEnv("ORDER_ASSURANCE_URL", "http://localhost:9090"),
		SyncJobEnabled:    getEnvAsBool("SYNC_JOB_ENABLED", true),
		SyncJobCron:       getEnv("SYNC_JOB_CRON", "0 * * * *"),
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}