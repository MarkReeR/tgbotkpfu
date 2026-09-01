package main

import (
	Bot "Bot"
	Config "Bot/Config"
	Database "Bot/Database"
	Logger "Bot/Logger"
	Schedule "Bot/Schedule"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/glebarez/sqlite"
	tgBotAPI "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// version is stamped at build time via -ldflags "-X main.version=..." so a
// running container can be tied back to the commit it was built from.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run holds the real body of main so that deferred cleanup (closing the log
// file) still happens on the error paths, which os.Exit would skip.
func run() error {
	cfg, err := Config.Load(Config.DefaultPath)
	if err != nil {
		return err
	}

	logFile, err := setupLogging(cfg.LogDir, cfg.LogLevel)
	if err != nil {
		return err
	}
	defer logFile.Close()
	Logger.Info("tgbotkpfu %s starting", version)

	botAPI, err := tgBotAPI.NewBotAPI(cfg.BotToken)
	if err != nil {
		return fmt.Errorf("не удалось подключиться к Telegram: %w", err)
	}
	Logger.Info("connected to Telegram as @%s", botAPI.Self.UserName)

	database, err := Database.InitDatabase(sqlite.Open(cfg.DatabasePath))
	if err != nil {
		return fmt.Errorf("не удалось открыть базу %s: %w", cfg.DatabasePath, err)
	}
	Logger.Info("database ready at %s", cfg.DatabasePath)

	sched := Schedule.NewCache(cfg.SpreadsheetID, cfg.Sheets)
	if err := sched.Refresh(); err != nil {
		Logger.Warn("schedule: initial load failed, will retry in background: %v", err)
	}
	sched.StartAutoRefresh(cfg.RefreshInterval())
	Logger.Info("schedule: auto-refresh every %d minutes", cfg.RefreshMinutes)

	physEd := Schedule.NewPhysEdCache(cfg.PhysEdSpreadsheetID, cfg.PhysEdGid)
	if physEd.Configured() {
		if err := physEd.Refresh(); err != nil {
			Logger.Warn("physed: initial load failed, will retry in background: %v", err)
		}
		physEd.StartAutoRefresh(cfg.RefreshInterval())
	} else {
		Logger.Warn("physed: no sheet configured, the Физра button will say so")
	}

	Logger.Info("exams button: %v", cfg.ShowExams)

	Logger.Info("semester starts %s, that week is the upper one",
		cfg.SemesterStart.Format("02.01.2006"))

	bot := Bot.NewBotService(botAPI, database, sched, physEd, Bot.Options{
		Location:    cfg.Location,
		Calendar:    cfg.Calendar(),
		ShowExams:   cfg.ShowExams,
		PhysEdVenue: cfg.PhysEdVenue,
		ScheduleURL: cfg.ScheduleURL(),
	})

	// Stop cleanly on Ctrl+C or docker stop, so the log file is flushed.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		Logger.Info("shutdown signal received")
		bot.Final()
	}()

	bot.Start()
	return nil
}

// setupLogging sends the log to both the console and a timestamped file in dir.
func setupLogging(dir string, level string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("не удалось создать папку для логов %s: %w", dir, err)
	}

	path := filepath.Join(dir, "bot_"+time.Now().Format("2006-01-02_15-04-05")+".log")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл лога: %w", err)
	}

	Logger.Setup(io.MultiWriter(os.Stdout, f), Logger.ParseLevel(level))
	Logger.Info("logging to %s at level %s", path, strings.ToUpper(level))
	return f, nil
}
