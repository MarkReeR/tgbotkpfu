package handler

import (
	"context"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kpfu-schedule-bot/go-bot/internal/domain"
	"github.com/kpfu-schedule-bot/go-bot/internal/logger"
	"github.com/kpfu-schedule-bot/go-bot/internal/service"
)

// StartHandler обрабатывает команду /start
type StartHandler struct {
	logger  *logger.Logger
	bot     *tgbotapi.BotAPI
	userSvc domain.UserService
}

// NewStartHandler создает новый экземпляр обработчика start
func NewStartHandler(logger *logger.Logger, bot *tgbotapi.BotAPI, userSvc domain.UserService) *StartHandler {
	return &StartHandler{logger: logger, bot: bot, userSvc: userSvc}
}

// Handle обрабатывает команду /start
func (h *StartHandler) Handle(update tgbotapi.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	h.logger.Info("User started bot", "user_id", msg.From.ID, "username", msg.From.UserName)

	// Создаем клавиатуру с кнопкой "Расписание" и "Настройки"
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📅 Расписание"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⚙️ Настройки"),
		),
	)
	keyboard.ResizeKeyboard = true

	responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
		"👋 Добро пожаловать в бот расписания КФУ!\n"+
			"Сейчас бот в режиме разработки, поэтому возможны перебои в работе.\n"+
			"Нажмите кнопку ниже, чтобы посмотреть расписание.")
	responseMsg.ReplyMarkup = keyboard
	responseMsg.ParseMode = ""

	if _, err := h.bot.Send(responseMsg); err != nil {
		h.logger.Error("Failed to send message", "error", err)
	}
}

// HandleScheduleButton обрабатывает нажатие кнопки "Расписание"
func (h *StartHandler) HandleScheduleButton(update tgbotapi.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	h.logger.Info("User requested schedule", "user_id", msg.From.ID)

	responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
		"Введите номер группы:\nПример: 8251160")
	responseMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)

	if _, err := h.bot.Send(responseMsg); err != nil {
		h.logger.Error("Failed to send message", "error", err)
	}
}

// ScheduleHandler обрабатывает запросы расписания
type ScheduleHandler struct {
	logger      *logger.Logger
	bot         *tgbotapi.BotAPI
	csvRepo     domain.CSVRepository
	scheduleSvc domain.ScheduleService
	userSvc     domain.UserService
}

// NewScheduleHandler создает новый экземпляр обработчика расписания
func NewScheduleHandler(
	logger *logger.Logger,
	bot *tgbotapi.BotAPI,
	csvRepo domain.CSVRepository,
	scheduleSvc domain.ScheduleService,
	userSvc domain.UserService,
) *ScheduleHandler {
	return &ScheduleHandler{
		logger:      logger,
		bot:         bot,
		csvRepo:     csvRepo,
		scheduleSvc: scheduleSvc,
		userSvc:     userSvc,
	}
}

// Handle обрабатывает ввод номера группы
func (h *ScheduleHandler) Handle(update tgbotapi.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	text := msg.Text
	if text == "" {
		return
	}

	// Игнорируем команды (начинаются с /)
	if strings.HasPrefix(text, "/") {
		return
	}

	h.logger.Info("User requested schedule by group", "user_id", msg.From.ID, "group", text)

	group := normalizeGroupCode(text)
	if len(group) != 7 {
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
			"❗Номер группы должен содержать ровно 7 цифр.\n"+
				"Попробуйте снова:")
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
		}
		return
	}

	// Отправляем сообщение о поиске
	statusMsg := tgbotapi.NewMessage(msg.Chat.ID, "🔍 Ищу группу "+group+"...")
	sentStatus, err := h.bot.Send(statusMsg)
	if err != nil {
		h.logger.Error("Failed to send status message", "error", err)
		return
	}

	// Находим расписание в кэше
	ctx := context.Background()
	csvText, err := h.csvRepo.FindGroupSchedule(ctx, group)
	if err != nil || csvText == "" {
		editMsg := tgbotapi.NewEditMessageText(msg.Chat.ID, sentStatus.MessageID,
			"❌ Группа <b>"+escapeHTML(group)+"</b> не найдена.\n"+
				"Проверьте правильность написания номера группы.")
		editMsg.ParseMode = "HTML"
		if _, err := h.bot.Send(editMsg); err != nil {
			h.logger.Error("Failed to edit message", "error", err)
		}
		return
	}

	// Парсим расписание
	lessons, err := h.scheduleSvc.ParseSchedule(csvText, group)
	if err != nil {
		h.logger.Error("Failed to parse schedule", "group", group, "error", err)
		editMsg := tgbotapi.NewEditMessageText(msg.Chat.ID, sentStatus.MessageID,
			"▲ Ошибка обработки: <code>"+escapeHTML(err.Error())+"</code>")
		editMsg.ParseMode = "HTML"
		if _, err := h.bot.Send(editMsg); err != nil {
			h.logger.Error("Failed to edit message", "error", err)
		}
		return
	}

	// Сохраняем в кэш пользователя и БД
	if err := h.userSvc.SetUserSchedule(ctx, msg.From.ID, group, lessons); err != nil {
		h.logger.Error("Failed to cache schedule", "error", err)
	}

	if len(lessons) == 0 {
		editMsg := tgbotapi.NewEditMessageText(msg.Chat.ID, sentStatus.MessageID,
			"ℹ️ Группа <b>"+escapeHTML(group)+"</b> найдена, но расписание пустое.\n"+
				"Возможно, на этой неделе нет занятий.")
		editMsg.ParseMode = "HTML"
		if _, err := h.bot.Send(editMsg); err != nil {
			h.logger.Error("Failed to edit message", "error", err)
		}
		return
	}

	// Удаляем статусное сообщение
	deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, sentStatus.MessageID)
	h.bot.Request(deleteMsg)

	// Отправляем сообщение с кнопками
	responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
		"✅ Группа <b>"+escapeHTML(group)+"</b> найдена!\n"+
			"Выберите период для просмотра:")
	responseMsg.ParseMode = "HTML"
	responseMsg.ReplyMarkup = getScheduleKeyboard()

	if _, err := h.bot.Send(responseMsg); err != nil {
		h.logger.Error("Failed to send message", "error", err)
	}
}

// ScheduleButtonsHandler обрабатывает кнопки расписания
type ScheduleButtonsHandler struct {
	logger      *logger.Logger
	bot         *tgbotapi.BotAPI
	scheduleSvc domain.ScheduleService
	userSvc     domain.UserService
}

// NewScheduleButtonsHandler создает новый экземпляр обработчика кнопок
func NewScheduleButtonsHandler(
	logger *logger.Logger,
	bot *tgbotapi.BotAPI,
	scheduleSvc domain.ScheduleService,
	userSvc domain.UserService,
) *ScheduleButtonsHandler {
	return &ScheduleButtonsHandler{
		logger:      logger,
		bot:         bot,
		scheduleSvc: scheduleSvc,
		userSvc:     userSvc,
	}
}

// Handle обрабатывает нажатия кнопок
func (h *ScheduleButtonsHandler) Handle(update tgbotapi.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	text := msg.Text
	userID := msg.From.ID

	h.logger.Info("User pressed button", "user_id", userID, "button", text)

	switch text {
	case "🔍 Другая группа":
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
			"Введите номер группы:\nПример: 09-825, 8251160, 8251")
		responseMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
		}
		return

	case "⬅️ Назад":
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID, "Выберите действие:")
		responseMsg.ReplyMarkup = getScheduleKeyboard()
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
		}
		return

	case "⚙️ Настройки":
		h.handleSettings(update)
		return

	case "🔔 Включить уведомления":
		h.handleToggleNotifications(update, true)
		return

	case "🔕 Выключить уведомления":
		h.handleToggleNotifications(update, false)
		return
	}

	// Получаем расписание из кэша
	ctx := context.Background()
	cached, err := h.userSvc.GetUserSchedule(ctx, userID)
	if err != nil || cached == nil {
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
			"Расписание не найдено или устарело. Введите группу снова:")
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
		}
		return
	}

	group := cached.Group
	lessons := cached.Lessons

	switch text {
	case "📅 Сегодня":
		dayName := service.GetDayName(0)
		dayLessons := h.scheduleSvc.FilterByDay(lessons, dayName)
		dayLessons = h.scheduleSvc.FilterByWeek(dayLessons, time.Now())
		formatted := h.scheduleSvc.FormatDaySchedule(dayLessons, dayName, false)
		
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID, formatted)
		responseMsg.ParseMode = "HTML"
		responseMsg.DisableWebPagePreview = true
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
		}

	case "📅 Завтра":
		dayName := service.GetDayName(1)
		dayLessons := h.scheduleSvc.FilterByDay(lessons, dayName)
		dayLessons = h.scheduleSvc.FilterByWeek(dayLessons, time.Now().AddDate(0, 0, 1))
		formatted := h.scheduleSvc.FormatDaySchedule(dayLessons, dayName, false)
		
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID, formatted)
		responseMsg.ParseMode = "HTML"
		responseMsg.DisableWebPagePreview = true
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
		}

	case "📋 Вся неделя":
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID, "Выберите фильтр:")
		responseMsg.ReplyMarkup = getWeekMenuKeyboard()
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
		}

	case "🔎 Текущая неделя":
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
			"📆 <b>Расписание на текущую неделю</b>\nГруппа: <b>"+escapeHTML(group)+"</b>")
		responseMsg.ParseMode = "HTML"
		responseMsg.ReplyMarkup = getWeekMenuKeyboard()
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
			return
		}

		daysOrder := []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}
		for _, day := range daysOrder {
			dayLessons := h.scheduleSvc.FilterByDay(lessons, day)
			dayLessons = h.scheduleSvc.FilterByWeek(dayLessons, time.Now())
			dayText := h.scheduleSvc.FormatDaySchedule(dayLessons, day, false)
			
			dayMsg := tgbotapi.NewMessage(msg.Chat.ID, dayText)
			dayMsg.ParseMode = "HTML"
			dayMsg.DisableWebPagePreview = true
			if _, err := h.bot.Send(dayMsg); err != nil {
				h.logger.Error("Failed to send day message", "error", err)
			}
			time.Sleep(100 * time.Millisecond) // Небольшая задержка между сообщениями
		}

	case "➡️ Следующая неделя":
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
			"📆 <b>Расписание на следующую неделю</b>\nГруппа: <b>"+escapeHTML(group)+"</b>")
		responseMsg.ParseMode = "HTML"
		responseMsg.ReplyMarkup = getWeekMenuKeyboard()
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
			return
		}

		targetDate := time.Now().AddDate(0, 0, 7)
		daysOrder := []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}
		for _, day := range daysOrder {
			dayLessons := h.scheduleSvc.FilterByDay(lessons, day)
			dayLessons = h.scheduleSvc.FilterByWeek(dayLessons, targetDate)
			dayText := h.scheduleSvc.FormatDaySchedule(dayLessons, day, false)
			
			dayMsg := tgbotapi.NewMessage(msg.Chat.ID, dayText)
			dayMsg.ParseMode = "HTML"
			dayMsg.DisableWebPagePreview = true
			if _, err := h.bot.Send(dayMsg); err != nil {
				h.logger.Error("Failed to send day message", "error", err)
			}
			time.Sleep(100 * time.Millisecond)
		}

	case "📚 Вся без фильтров":
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
			"📆 <b>Расписание на неделю (без фильтра)</b>\nГруппа: <b>"+escapeHTML(group)+"</b>")
		responseMsg.ParseMode = "HTML"
		responseMsg.ReplyMarkup = getWeekMenuKeyboard()
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
			return
		}

		daysOrder := []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}
		for _, day := range daysOrder {
			dayLessons := h.scheduleSvc.FilterByDay(lessons, day)
			dayText := h.scheduleSvc.FormatDaySchedule(dayLessons, day, true)
			
			dayMsg := tgbotapi.NewMessage(msg.Chat.ID, dayText)
			dayMsg.ParseMode = "HTML"
			dayMsg.DisableWebPagePreview = true
			if _, err := h.bot.Send(dayMsg); err != nil {
				h.logger.Error("Failed to send day message", "error", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// handleSettings обрабатывает кнопку настроек
func (h *ScheduleButtonsHandler) handleSettings(update tgbotapi.Update) {
	msg := update.Message
	ctx := context.Background()
	
	user, err := h.userSvc.GetUser(ctx, msg.From.ID)
	if err != nil {
		h.logger.Error("Failed to get user", "error", err)
		return
	}

	statusText := "🔕 Выключены"
	buttonText := "🔔 Включить уведомления"
	if user.NotificationsEnabled {
		statusText = "🔔 Включены"
		buttonText = "🔕 Выключить уведомления"
	}

	groupText := "Не привязана"
	if user.Group != "" {
		groupText = user.Group
	}

	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(buttonText),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Назад"),
		),
	)
	keyboard.ResizeKeyboard = true

	responseMsg := tgbotapi.NewMessage(msg.Chat.ID,
		"⚙️ <b>Настройки</b>\n\n"+
			"📚 Группа: <b>"+escapeHTML(groupText)+"</b>\n"+
			"🔔 Уведомления: <b>"+statusText+"</b>\n\n"+
			"Уведомления приходят за час до начала первой пары.")
	responseMsg.ParseMode = "HTML"
	responseMsg.ReplyMarkup = keyboard

	if _, err := h.bot.Send(responseMsg); err != nil {
		h.logger.Error("Failed to send message", "error", err)
	}
}

// handleToggleNotifications переключает статус уведомлений
func (h *ScheduleButtonsHandler) handleToggleNotifications(update tgbotapi.Update, enabled bool) {
	msg := update.Message
	ctx := context.Background()

	if err := h.userSvc.SetNotifications(ctx, msg.From.ID, enabled); err != nil {
		h.logger.Error("Failed to set notifications", "error", err)
		responseMsg := tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при обновлении настроек")
		if _, err := h.bot.Send(responseMsg); err != nil {
			h.logger.Error("Failed to send message", "error", err)
		}
		return
	}

	statusText := "выключены"
	if enabled {
		statusText = "включены"
	}

	responseMsg := tgbotapi.NewMessage(msg.Chat.ID, 
		"✅ Уведомления "+statusText+".\nТеперь вы будете получать расписание за час до первой пары.")
	responseMsg.ReplyMarkup = getScheduleKeyboard()
	if _, err := h.bot.Send(responseMsg); err != nil {
		h.logger.Error("Failed to send message", "error", err)
	}
}

// getScheduleKeyboard возвращает клавиатуру с кнопками расписания
func getScheduleKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📅 Сегодня"),
			tgbotapi.NewKeyboardButton("📅 Завтра"),
			tgbotapi.NewKeyboardButton("📋 Вся неделя"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔍 Другая группа"),
		),
	)
}

// getWeekMenuKeyboard возвращает клавиатуру меню недели
func getWeekMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔎 Текущая неделя"),
			tgbotapi.NewKeyboardButton("➡️ Следующая неделя"),
			tgbotapi.NewKeyboardButton("📚 Вся без фильтров"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Назад"),
		),
	)
}

// normalizeGroupCode нормализует код группы
func normalizeGroupCode(s string) string {
	var result strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			result.WriteRune(ch)
		}
	}
	return result.String()
}

// escapeHTML экранирует HTML-символы
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
