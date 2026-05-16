package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kpfu-schedule-bot/go-bot/internal/config"
	"github.com/kpfu-schedule-bot/go-bot/internal/handler"
	"github.com/kpfu-schedule-bot/go-bot/internal/infrastructure"
	"github.com/kpfu-schedule-bot/go-bot/internal/service"
)

func main() {
	ctx := context.Background()

	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Настраиваем логгер
	logger := setupLogger(cfg)
	logger.Info("Starting bot...")

	// Создаем CSV репозиторий
	csvRepo, err := infrastructure.NewCSVRepository(cfg, logger.Logger)
	if err != nil {
		logger.Error("Failed to create CSV repository", "error", err)
		os.Exit(1)
	}

	// Гарантируем наличие кэша при старте
	if err := csvRepo.EnsureStartupCache(ctx); err != nil {
		logger.Error("Failed to ensure startup cache", "error", err)
		os.Exit(1)
	}

	// Строим индекс групп
	if err := csvRepo.BuildIndex(); err != nil {
		logger.Error("Failed to build group index", "error", err)
		os.Exit(1)
	}

	// Создаем SQLite репозиторий пользователей
	userRepo, err := infrastructure.NewSQLiteUserRepository("data/users.db", logger.Logger)
	if err != nil {
		logger.Error("Failed to create user repository", "error", err)
		os.Exit(1)
	}
	defer userRepo.Close()

	// Создаем сервисы
	scheduleSvc := service.NewScheduleService(logger.Logger)
	cacheSvc := service.NewMemoryCacheService()
	userSvc := service.NewUserService(cacheSvc, userRepo)

	// Создаем бота
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		logger.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}
	logger.Info("Bot authorized", "username", bot.Self.UserName)

	// Создаем обработчики
	startHandler := handler.NewStartHandler(logger.Logger, bot, userSvc)
	scheduleHandler := handler.NewScheduleHandler(logger.Logger, bot, csvRepo, scheduleSvc, userSvc)
	buttonsHandler := handler.NewScheduleButtonsHandler(logger.Logger, bot, scheduleSvc, userSvc)

	// Создаем канал для обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Создаем каналы для управления
	shutdown := make(chan struct{})
	var wg sync.WaitGroup

	// Запускаем планировщик обновлений
	wg.Add(1)
	go func() {
		defer wg.Done()
		runRefreshScheduler(ctx, shutdown, csvRepo, logger.Logger, cfg)
	}()

	// Запускаем планировщик уведомлений
	wg.Add(1)
	go func() {
		defer wg.Done()
		runNotificationScheduler(ctx, shutdown, bot, userRepo, csvRepo, scheduleSvc, logger.Logger, cfg)
	}()

	// Обрабатываем обновления
	logger.Info("Started polling...")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case update := <-updates:
			handleUpdate(update, startHandler, scheduleHandler, buttonsHandler, logger.Logger)

		case sig := <-sigChan:
			logger.Info("Received signal, shutting down", "signal", sig)
			close(shutdown)
			wg.Wait()
			bot.StopReceivingUpdates()
			logger.Info("Bot stopped")
			return
		}
	}
}

// handleUpdate обрабатывает одно обновление от Telegram
func handleUpdate(
	update tgbotapi.Update,
	startHandler *handler.StartHandler,
	scheduleHandler *handler.ScheduleHandler,
	buttonsHandler *handler.ScheduleButtonsHandler,
	logger *log.Logger,
) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	text := msg.Text

	// Команда /start
	if msg.IsCommand() && msg.Command() == "start" {
		startHandler.Handle(update)
		return
	}

	// Кнопка "📅 Расписание"
	if text == "📅 Расписание" {
		startHandler.HandleScheduleButton(update)
		return
	}

	// Кнопки расписания
	if isScheduleButton(text) {
		buttonsHandler.Handle(update)
		return
	}

	// Ввод номера группы (текст без слэша)
	if text != "" && !msg.IsCommand() {
		scheduleHandler.Handle(update)
		return
	}
}

// isScheduleButton проверяет, является ли текст кнопкой расписания
func isScheduleButton(text string) bool {
	buttons := []string{
		"📅 Сегодня", "📅 Завтра", "📋 Вся неделя", "🔍 Другая группа",
		"🔎 Текущая неделя", "➡️ Следующая неделя", "📚 Вся без фильтров", "⬅️ Назад",
	}
	for _, btn := range buttons {
		if text == btn {
			return true
		}
	}
	return false
}

// runRefreshScheduler запускает планировщик периодического обновления CSV
func runRefreshScheduler(
	ctx context.Context,
	shutdown <-chan struct{},
	csvRepo *infrastructure.CSVRepositoryImpl,
	logger *log.Logger,
	cfg *config.Config,
) {
	for {
		select {
		case <-shutdown:
			logger.Println("Refresh scheduler stopped")
			return
		default:
		}

		// Вычисляем время до следующего обновления
		secs := secondsUntilNextRefresh(cfg.RefreshAt, cfg.TZ)
		logger.Printf("Next CSV refresh in %f seconds", secs)

		// Ждем или сигнала остановки, или времени обновления
		select {
		case <-shutdown:
			logger.Println("Refresh scheduler stopped")
			return
		case <-time.After(time.Duration(secs) * time.Second):
		}

		// Обновляем CSV
		if err := csvRepo.RefreshAll(ctx); err != nil {
			logger.Printf("Failed to refresh CSV: %v", err)
			time.Sleep(time.Minute) // Пауза перед повторной попыткой
		}
	}
}

// secondsUntilNextRefresh вычисляет количество секунд до следующего запланированного времени
func secondsUntilNextRefresh(times []time.Time, loc *time.Location) float64 {
	now := time.Now().In(loc)
	var nextTime time.Time

	for _, t := range times {
		// Привязываем время к сегодняшнему дню
		today := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, loc)
		if today.After(now) {
			if nextTime.IsZero() || today.Before(nextTime) {
				nextTime = today
			}
		}
	}

	// Если все времена сегодня уже прошли, берем первое на завтра
	if nextTime.IsZero() {
		tomorrow := now.AddDate(0, 0, 1)
		for _, t := range times {
			candidate := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), t.Hour(), t.Minute(), 0, 0, loc)
			if nextTime.IsZero() || candidate.Before(nextTime) {
				nextTime = candidate
			}
		}
	}

	return nextTime.Sub(now).Seconds()
}

// runNotificationScheduler запускает планировщик ежедневных уведомлений
func runNotificationScheduler(
	ctx context.Context,
	shutdown <-chan struct{},
	bot *tgbotapi.BotAPI,
	userRepo domain.UserRepository,
	csvRepo domain.CSVRepository,
	scheduleSvc domain.ScheduleService,
	logger *log.Logger,
	cfg *config.Config,
) {
	logger.Info("Notification scheduler started")

	for {
		select {
		case <-shutdown:
			logger.Println("Notification scheduler stopped")
			return
		default:
		}

		// Вычисляем время до следующего запуска (за час до первой пары - 8:00, т.е. в 7:00)
		notifyTime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 7, 0, 0, 0, cfg.TZ)
		now := time.Now().In(cfg.TZ)

		var waitSeconds float64
		if notifyTime.After(now) {
			waitSeconds = notifyTime.Sub(now).Seconds()
		} else {
			// Если уже прошло, ждем до завтра
			notifyTime = notifyTime.AddDate(0, 0, 1)
			waitSeconds = notifyTime.Sub(now).Seconds()
		}

		logger.Printf("Next notification check in %f seconds", waitSeconds)

		select {
		case <-shutdown:
			logger.Println("Notification scheduler stopped")
			return
		case <-time.After(time.Duration(waitSeconds) * time.Second):
		}

		// Отправляем уведомления
		sendDailyNotifications(ctx, bot, userRepo, csvRepo, scheduleSvc, logger, cfg.TZ)
	}
}

// sendDailyNotifications отправляет ежедневные уведомления пользователям
func sendDailyNotifications(
	ctx context.Context,
	bot *tgbotapi.BotAPI,
	userRepo domain.UserRepository,
	csvRepo domain.CSVRepository,
	scheduleSvc domain.ScheduleService,
	logger *log.Logger,
	loc *time.Location,
) {
	logger.Info("Sending daily notifications...")

	// Получаем всех пользователей с включенными уведомлениями
	users, err := userRepo.GetWithNotifications(ctx)
	if err != nil {
		logger.Error("Failed to get users with notifications", "error", err)
		return
	}

	if len(users) == 0 {
		logger.Info("No users with notifications enabled")
		return
	}

	today := time.Now().In(loc)
	dayName := service.GetDayName(0)

	for _, user := range users {
		if user.Group == "" {
			continue // Пропускаем пользователей без привязанной группы
		}

		// Получаем расписание для группы пользователя
		csvText, err := csvRepo.FindGroupSchedule(ctx, user.Group)
		if err != nil || csvText == "" {
			logger.Warn("Failed to get schedule for user", "user_id", user.ID, "group", user.Group)
			continue
		}

		lessons, err := scheduleSvc.ParseSchedule(csvText, user.Group)
		if err != nil {
			logger.Error("Failed to parse schedule for notification", "user_id", user.ID, "error", err)
			continue
		}

		// Фильтруем по дню и неделе
		dayLessons := scheduleSvc.FilterByDay(lessons, dayName)
		dayLessons = scheduleSvc.FilterByWeek(dayLessons, today)

		if len(dayLessons) == 0 {
			// Нет пар сегодня
			continue
		}

		formatted := scheduleSvc.FormatDaySchedule(dayLessons, dayName, false)

		msg := tgbotapi.NewMessage(user.ID, 
			"🔔 <b>Расписание на сегодня</b>\n\n"+formatted)
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true

		if _, err := bot.Send(msg); err != nil {
			logger.Error("Failed to send notification", "user_id", user.ID, "error", err)
		}

		// Небольшая задержка между сообщениями
		time.Sleep(100 * time.Millisecond)
	}

	logger.Info("Daily notifications sent", "count", len(users))
}

// setupLogger настраивает логгер
func setupLogger(cfg *config.Config) *log.Logger {
	// Создаем директорию для логов
	dir := filepath.Dir(cfg.LogFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	// Открываем файл для логов
	file, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}

	// Создаем логгер с выводом в файл и консоль
	mw := io.MultiWriter(file, os.Stdout)
	return log.New(mw, "", log.LstdFlags|log.Lmicroseconds)
}
