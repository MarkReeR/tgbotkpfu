package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config хранит все настройки приложения
type Config struct {
	BotToken         string
	SpreadsheetID    string
	GIDs             []int
	CacheDir         string
	RefreshAt        []time.Time
	TZ               *time.Location
	LogLevel         string
	LogFile          string
	APIAdminLogin    string
	APIAdminPassword string
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	// Загружаем .env файл (если используется godotenv)
	// godotenv.Load()

	tzName := getEnv("TZ", "Europe/Moscow")
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone: %w", err)
	}

	refreshAt, err := parseRefreshTimes(getEnv("REFRESH_AT", "04:00,19:00"), loc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse refresh times: %w", err)
	}

	gids, err := parseGIDs(getEnv("GIDS", "0,1,2,3,4,5"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse GIDs: %w", err)
	}

	return &Config{
		BotToken:         getEnv("BOT_TOKEN", ""),
		SpreadsheetID:    getEnv("SPREADSHEET_ID", ""),
		GIDs:             gids,
		CacheDir:         getEnv("CACHE_DIR", "data/csv"),
		RefreshAt:        refreshAt,
		TZ:               loc,
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		LogFile:          getEnv("LOG_FILE", "logs/bot.log"),
		APIAdminLogin:    getEnv("API_ADMIN_LOGIN", ""),
		APIAdminPassword: getEnv("API_ADMIN_PASSWORD", ""),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseGIDs(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	var result []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		val, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid gid %q: %w", p, err)
		}
		result = append(result, val)
	}
	return result, nil
}

func parseRefreshTimes(raw string, loc *time.Location) ([]time.Time, error) {
	var result []time.Time
	parts := strings.Split(raw, ",")
	now := time.Now().In(loc)

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) != 5 || p[2] != ':' {
			continue
		}

		hour, err := strconv.Atoi(p[:2])
		if err != nil {
			continue
		}
		minute, err := strconv.Atoi(p[3:])
		if err != nil {
			continue
		}

		t := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
		result = append(result, t)
	}

	if len(result) == 0 {
		// Значения по умолчанию
		t1 := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, loc)
		t2 := time.Date(now.Year(), now.Month(), now.Day(), 19, 0, 0, 0, loc)
		result = []time.Time{t1, t2}
	}

	return result, nil
}
