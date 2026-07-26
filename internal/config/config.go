package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DBDriver      string
	SQLitePath    string
	DBHost        string
	DBPort        int
	DBUser        string
	DBPassword    string
	DBName        string
	DBSSLMode     string
	ServerPort    string
	Timezone      string
	AllowedOrigin string
	DBMaxOpen     int
	DBMaxIdle     int
	DBMaxLifetime time.Duration
}

func LoadConfig() *Config {
	_ = godotenv.Load()
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	maxOpen, _ := strconv.Atoi(getEnv("DB_MAX_OPEN", "20"))
	maxIdle, _ := strconv.Atoi(getEnv("DB_MAX_IDLE", "5"))
	maxLifetime, _ := time.ParseDuration(getEnv("DB_MAX_LIFETIME", "30m"))
	return &Config{
		DBDriver:      getEnv("DB_DRIVER", "postgres"),
		SQLitePath:    getEnv("SQLITE_PATH", "restaurant-finance.db"),
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        dbPort,
		DBUser:        getEnv("DB_USER", "postgres"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBName:        getEnv("DB_NAME", "restaurant_finance"),
		DBSSLMode:     getEnv("DB_SSLMODE", "disable"),
		ServerPort:    getEnv("SERVER_PORT", "8080"),
		Timezone:      getEnv("APP_TIMEZONE", "Europe/Moscow"),
		AllowedOrigin: getEnv("ALLOWED_ORIGIN", "http://localhost:5173"),
		DBMaxOpen:     maxOpen,
		DBMaxIdle:     maxIdle,
		DBMaxLifetime: maxLifetime,
	}
}

func LoadDesktopConfig() (*Config, error) {
	cfg := LoadConfig()
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dataDir := filepath.Join(configDir, "RestaurantFinance")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	cfg.DBDriver = "sqlite"
	cfg.SQLitePath = filepath.Join(dataDir, "restaurant-finance.db")
	cfg.DBMaxOpen = 4
	cfg.DBMaxIdle = 2
	cfg.DBMaxLifetime = 0
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
