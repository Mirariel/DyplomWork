package config

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort   string
	AppDebug  bool
	JWTSecret string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	EncryptionKey string
	AnthropicKey  string

	// RedisURL — необов'язковий. Якщо порожній — використовується in-memory cache.
	// Формат: "localhost:6379" або "redis://:password@host:6379/0"
	RedisURL string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using environment variables")
	}

	return &Config{
		AppPort:       getEnv("APP_PORT", "8080"),
		AppDebug:      getEnvBool("APP_DEBUG", false),
		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),

		DBHost:        getEnv("DB_HOST", "127.0.0.1"),
		DBPort:        getEnv("DB_PORT", "3306"),
		DBUser:        getEnv("DB_USER", "root"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBName:        getEnv("DB_NAME", "tradetracker"),

		EncryptionKey: getEnv("APP_KEY", ""),
		AnthropicKey:  getEnv("ANTHROPIC_API_KEY", ""),
		RedisURL:      getEnv("REDIS_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
