package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"tgbotkpfu/internal/config"
	"tgbotkpfu/internal/handler"
	"tgbotkpfu/internal/repository"
	"tgbotkpfu/internal/service"
)

func main() {
	cfg := config.Load()

	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Panicf("Ошибка инициализации бота: %v", err)
	}

	log.Printf("Бот авторизован как %s", bot.Self.UserName)

	repo, err := repository.NewFileRepository(cfg.DBPath)
	if err != nil {
		log.Panicf("Ошибка инициализации репозитория: %v", err)
	}

	userService := service.NewUserService(repo)
	h := handler.NewHandler(bot, userService)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	h.ScheduleNotificationTask()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case update := <-updates:
			if update.Message != nil {
				switch update.Message.Command() {
				case "start":
					h.HandleStart(update)
				case "bind":
					h.HandleBind(update)
				case "schedule":
					h.HandleSchedule(update)
				case "settings":
					h.HandleSettings(update)
				default:
					h.HandleStart(update)
				}
			} else if update.CallbackQuery != nil {
				h.HandleCallback(update)
			}

		case <-sigChan:
			log.Println("Получен сигнал завершения, выход...")
			return
		}
	}
}
