package config

import (
	"os"
)

var EnableWebsocketIncomingMessage bool
var EnableWebhook bool
var WarmingWorkerEnabled bool
var WarmingAutoReplyEnabled bool
var WarmingAutoReplyCooldown int // seconds

// AI Configuration
var AIEnabled bool
var AIDefaultProvider string
var GeminiAPIKey string
var GeminiDefaultModel string
var AIConversationHistoryLimit int
var AIDefaultTemperature float64
var AIDefaultMaxTokens int

type Config struct {
	Port               string
	DBConnectionString string
}

func Load() *Config {
	return &Config{
		Port:               getEnv("PORT", "2121"),
		DBConnectionString: getEnv("DATABASE_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
