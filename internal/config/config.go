package config

import (
	"os"
	"strconv"
)

type Config struct {
	BotToken string
	DBPath   string
	AdminID  int64
}

func Load() *Config {
	return &Config{
		BotToken: getEnv("BOT_TOKEN", ""),
		DBPath:   getEnv("DB_PATH", "bot_data.db"),
		AdminID:  getEnvInt64("ADMIN_ID", 0),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		result, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return result
		}
	}
	return defaultValue
}
