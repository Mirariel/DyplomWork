package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"

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

	// Telegram — необов'язкові. Якщо порожні — сповіщення вимкнені.
	TelegramToken  string
	TelegramChatID string

	// NATS — брокер повідомлень між сервісами.
	// Формат: "nats://host:4222"
	NatsURL string

	// Внутрішні URL сервісів (використовуються api-gateway для проксування).
	MarketDataURL string
	TradingURL    string
	AnalyticsURL  string

	// Sync intervals — market-data scheduler.
	SyncLiveInterval time.Duration // lightweight sync for active WS users (positions+balances)
	SyncDeepInterval time.Duration // full sync for all users (history+spot trades)
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

		TelegramToken:  getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID: getEnv("TELEGRAM_CHAT_ID", ""),

		NatsURL: getEnv("NATS_URL", "nats://localhost:4222"),

		MarketDataURL: getEnv("MARKET_DATA_URL", "http://localhost:8081"),
		TradingURL:    getEnv("TRADING_URL",     "http://localhost:8082"),
		AnalyticsURL:  getEnv("ANALYTICS_URL",   "http://localhost:8083"),

		SyncLiveInterval: getEnvDuration("SYNC_LIVE_INTERVAL", 20*time.Second),
		SyncDeepInterval: getEnvDuration("SYNC_DEEP_INTERVAL", 15*time.Minute),
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

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
