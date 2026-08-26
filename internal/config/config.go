package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Env         string
	ServerPort  string
	DatabaseURL string
	RedisURL    string
	KafkaBroker string
	JWTSecret   string
}

func LoadConfig() *Config {
	// Load .env if present (ignore error if missing, rely on env vars)
	_ = godotenv.Load()

	cfg := &Config{
		Env:         getEnv("ENV", "development"),
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379/0"),
		KafkaBroker: getEnv("KAFKA_BROKER", "localhost:9092"),
		JWTSecret:   getEnv("JWT_SECRET", "super-secret-key"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}


