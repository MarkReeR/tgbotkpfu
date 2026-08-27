// Package Config loads the bot's settings from config.ini, letting environment
// variables override any of them. The env layer is what makes the container
// image usable without baking a config file (and a bot token) into it.
package Config

import (
	Schedule "Bot/Schedule"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/ini.v1"

	_ "time/tzdata" // embeds the tz database so Europe/Moscow resolves on any host
)

// DefaultPath is where the config file is looked for unless CONFIG_PATH says otherwise.
const DefaultPath = "config.ini"

type Config struct {
	BotToken     string
	LogDir       string
	LogLevel     string
	DatabasePath string
	Location     *time.Location

	SpreadsheetID  string
	Sheets         []Schedule.SheetSource
	RefreshMinutes int

	PhysEdSpreadsheetID string
	PhysEdGid           string
	PhysEdVenue         string

	ShowExams bool
}

// Load reads the config file, applies environment overrides and validates the
// result. A missing file is not fatal as long as the environment supplies what
// is required, which is the normal case in Docker.
func Load(path string) (Config, error) {
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		path = envPath
	}

	file, err := ini.Load(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("не удалось прочитать %s: %w", path, err)
		}
		file = ini.Empty() // env-only configuration
	}

	run := file.Section("runArgs")
	sched := file.Section("schedule")
	phys := file.Section("physed")
	exams := file.Section("exams")

	cfg := Config{
		BotToken:     env("BOT_TOKEN", run.Key("BotKey").String()),
		LogDir:       env("LOG_DIR", run.Key("LogDir").MustString("logs")),
		LogLevel:     env("LOG_LEVEL", run.Key("LogLevel").MustString("INFO")),
		DatabasePath: env("DATABASE_PATH", run.Key("DatabasePath").MustString("Database.db")),

		SpreadsheetID:  env("SPREADSHEET_ID", sched.Key("SpreadsheetId").String()),
		RefreshMinutes: envInt("REFRESH_MINUTES", sched.Key("RefreshMinutes").MustInt(15)),

		PhysEdSpreadsheetID: env("PHYSED_SPREADSHEET_ID", phys.Key("SpreadsheetId").String()),
		PhysEdGid:           env("PHYSED_GID", phys.Key("Gid").MustString("0")),
		PhysEdVenue:         env("PHYSED_VENUE", phys.Key("Venue").String()),

		ShowExams: envBool("EXAMS_SHOW", exams.Key("Show").MustBool(false)),
	}

	cfg.Sheets = ParseSheetSources(env("SHEETS", sched.Key("Sheets").String()))
	cfg.Location = loadLocation(env("TZ", run.Key("Timezone").MustString("Europe/Moscow")))

	return cfg, cfg.validate()
}

func (c Config) validate() error {
	switch {
	case strings.TrimSpace(c.BotToken) == "":
		return errors.New("не задан токен бота: укажите BotKey в config.ini или переменную BOT_TOKEN")
	case c.SpreadsheetID == "":
		return errors.New("не задан SpreadsheetId расписания")
	case len(c.Sheets) == 0:
		return errors.New(`не заданы листы расписания (Sheets), ожидается "1 курс:0,2 курс:123..."`)
	case c.RefreshMinutes < 1:
		return errors.New("RefreshMinutes должен быть больше нуля")
	}
	return nil
}

// RefreshInterval is how often the spreadsheets are re-downloaded.
func (c Config) RefreshInterval() time.Duration {
	return time.Duration(c.RefreshMinutes) * time.Minute
}

// loadLocation falls back to a fixed UTC+3 zone rather than failing, since an
// unknown zone name should not stop the bot from starting.
func loadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

// ParseSheetSources parses a "Name:Gid,Name:Gid,..." value into sheet sources.
func ParseSheetSources(raw string) []Schedule.SheetSource {
	var sources []Schedule.SheetSource
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, gid, found := strings.Cut(part, ":")
		if !found {
			continue
		}
		name, gid = strings.TrimSpace(name), strings.TrimSpace(gid)
		if name == "" || gid == "" {
			continue
		}
		sources = append(sources, Schedule.SheetSource{Name: name, Gid: gid})
	}
	return sources
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return fallback
}
