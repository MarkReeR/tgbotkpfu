package handler

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"tgbotkpfu/internal/service"
)

type Handler struct {
	bot     *tgbotapi.BotAPI
	service *service.UserService
}

func NewHandler(bot *tgbotapi.BotAPI, svc *service.UserService) *Handler {
	return &Handler{bot: bot, service: svc}
}

func (h *Handler) HandleStart(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
		"Привет! Я бот для просмотра расписания КФУ.\n\n"+
			"Команды:\n"+
			"/bind <группа> - привязать группу\n"+
			"/schedule - посмотреть расписание на сегодня\n"+
			"/settings - настройки уведомлений")
	h.bot.Send(msg)
}

func (h *Handler) HandleBind(update tgbotapi.Update) {
	args := strings.Fields(update.Message.Text)
	if len(args) < 2 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
			"Использование: /bind <номер_группы>\nПример: /bind ИВТ-1-21")
		h.bot.Send(msg)
		return
	}

	groupID := args[1]
	err := h.service.BindUserToGroup(update.Message.Chat.ID, groupID)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
			fmt.Sprintf("Ошибка при привязке группы: %v", err))
		h.bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
		fmt.Sprintf("Группа %s успешно привязана!", groupID))
	h.bot.Send(msg)
}

func (h *Handler) HandleSchedule(update tgbotapi.Update) {
	scheduleText, err := h.service.GetScheduleForUser(update.Message.Chat.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
			fmt.Sprintf("Ошибка: %v", err))
		h.bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, scheduleText)
	h.bot.Send(msg)
}

func (h *Handler) HandleSettings(update tgbotapi.Update) {
	user, err := h.service.GetUser(update.Message.Chat.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
			fmt.Sprintf("Ошибка: %v", err))
		h.bot.Send(msg)
		return
	}

	status := "включены"
	if !user.NotifyEnabled {
		status = "выключены"
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("Переключить (%s)", status),
				"toggle_notify"),
		),
	)

	msg := tgbotapi.NewMessage(update.Message.Chat.ID,
		fmt.Sprintf("Настройки уведомлений\n\n"+
			"Ежедневные уведомления о расписании сейчас %s.\n"+
			"Уведомления приходят за час до первой пары.", status))
	msg.ReplyMarkup = keyboard
	h.bot.Send(msg)
}

func (h *Handler) HandleCallback(update tgbotapi.Update) {
	data := update.CallbackQuery.Data
	
	switch data {
	case "toggle_notify":
		user, err := h.service.GetUser(update.Message.Chat.ID)
		if err != nil {
			h.bot.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, "Ошибка"))
			return
		}

		newStatus := !user.NotifyEnabled
		err = h.service.ToggleNotify(update.Message.Chat.ID, newStatus)
		if err != nil {
			h.bot.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, "Ошибка при сохранении"))
			return
		}

		status := "включены"
		if !newStatus {
			status = "выключены"
		}

		h.bot.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, 
			fmt.Sprintf("Уведомления %s", status)))

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("Переключить (%s)", status),
					"toggle_notify"),
			),
		)

		editMsg := tgbotapi.NewEditMessageText(update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			fmt.Sprintf("Настройки уведомлений\n\n"+
				"Ежедневные уведомления о расписании сейчас %s.\n"+
				"Уведомления приходят за час до первой пары.", status))
		editMsg.ReplyMarkup = &keyboard
		h.bot.Send(editMsg)
	}
}

func (h *Handler) SendDailyNotification(chatID int64, scheduleText string) {
	msg := tgbotapi.NewMessage(chatID, 
		"🔔 Расписание на сегодня:\n\n"+scheduleText)
	h.bot.Send(msg)
}

func (h *Handler) ScheduleNotificationTask() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			now := time.Now()
			
			if now.Weekday() == time.Sunday {
				continue
			}

			if now.Hour() == 7 && now.Minute() == 0 {
				h.sendNotificationsForTime(8, 0)
			}
		}
	}()
}

func (h *Handler) sendNotificationsForTime(targetHour, targetMinute int) {
	users, err := h.service.GetAllUsersWithNotify()
	if err != nil {
		log.Printf("Ошибка получения пользователей: %v", err)
		return
	}

	for _, user := range users {
		scheduleText, err := h.service.GetScheduleForUser(user.ID)
		if err != nil || scheduleText == "" {
			continue
		}

		h.SendDailyNotification(user.ID, scheduleText)
		time.Sleep(100 * time.Millisecond) // Небольшая задержка между отправками
	}
}
