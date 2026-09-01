package Bot

import (
	Logger "Bot/Logger"
	Schedule "Bot/Schedule"
	"fmt"
	"html"
	"strings"
	"time"
)

const groupCodeLength = 7

const welcomeText = "🎓 <b>Расписание ВТШ КФУ</b>\n\n" +
	"Пары на сегодня, завтра или всю неделю.\n" +
	"Выбранная группа запоминается — сменить её можно кнопкой ниже."

// handleCommand routes one incoming text message. Reply-keyboard buttons arrive
// as ordinary messages, so button captions are matched here just like commands.
func (bs *BotService) handleCommand(chatID int64, text string) {
	text = strings.TrimSpace(text)

	switch text {
	case "/start":
		bs.onStart(chatID)
	case "/help":
		bs.onHelp(chatID)

	// Main menu
	case BtnSchedule:
		bs.onScheduleButton(chatID)
	case BtnPhysEd:
		bs.onPhysEd(chatID)
	case BtnExams:
		bs.send(chatID, "📝 <b>Экзамены</b>\n\nРасписание экзаменов пока не опубликовано.", bs.mainKeyboard())
	case BtnChangeGroup:
		bs.askForGroup(chatID)
	case BtnMainMenu:
		bs.send(chatID, "Главное меню:", bs.mainKeyboard())

	// Schedule submenu
	case BtnToday:
		bs.onDay(chatID, 0)
	case BtnTomorrow:
		bs.onDay(chatID, 1)
	case BtnWeek:
		bs.send(chatID, "Выберите фильтр:", weekKeyboard())

	// Week submenu
	case BtnCurrentWeek:
		bs.onWeek(chatID, bs.now(), true, "Расписание на текущую неделю")
	case BtnNextWeek:
		bs.onWeek(chatID, bs.now().AddDate(0, 0, 7), true, "Расписание на следующую неделю")
	case BtnFullWeek:
		bs.onWeek(chatID, bs.now(), false, "Расписание на неделю (без фильтра)")
	case BtnBack:
		bs.send(chatID, "Выберите период:", scheduleKeyboard())

	default:
		if strings.HasPrefix(text, "/") {
			bs.send(chatID, "Неизвестная команда. Наберите /start.", nil)
			return
		}
		// Anything else is treated as a group number.
		bs.onGroupInput(chatID, text)
	}
}

// handleCallbackData routes a press on an inline button. messageID is the
// message the button belongs to, so day paging can rewrite it in place.
func (bs *BotService) handleCallbackData(chatID int64, messageID int, data string) {
	switch {
	case data == CallbackChangeGroup:
		// A fresh action, so clear what is on screen first. Day paging below
		// deliberately does not, since it rewrites the very message it is on.
		bs.clearTransient(chatID)
		bs.askForGroup(chatID)

	case data == CallbackMainMenu:
		// The "☰" under a day card: drop the card and show the main menu.
		bs.clearTransient(chatID)
		bs.send(chatID, "Главное меню:", bs.mainKeyboard())

	case strings.HasPrefix(data, CallbackDayPrefix):
		day, err := time.ParseInLocation(dateLayout, strings.TrimPrefix(data, CallbackDayPrefix), bs.Location)
		if err != nil {
			Logger.Warn("chat %d: bad day callback %q: %v", chatID, data, err)
			return
		}
		bs.showDayInPlace(chatID, messageID, day)

	default:
		Logger.Warn("chat %d: unknown callback %q", chatID, data)
	}
}

func (bs *BotService) now() time.Time {
	return time.Now().In(bs.Location)
}

// mainKeyboard renders the top-level menu with the exams button configured.
func (bs *BotService) mainKeyboard() interface{} {
	return mainKeyboard(bs.ShowExams)
}

// onStart resets the chat: it wipes the clutter and makes sure the welcome
// message is present, then shows the main menu.
func (bs *BotService) onStart(chatID int64) {
	bs.clearTransient(chatID)
	bs.ensureAnchor(chatID, true)

	if g, ok := bs.savedGroup(chatID); ok {
		bs.send(chatID, fmt.Sprintf("Ваша группа: <b>%s</b>", html.EscapeString(g.DisplayName())), bs.mainKeyboard())
		return
	}
	bs.send(chatID, "Для начала выберите группу — нажмите «"+BtnSchedule+"».", bs.mainKeyboard())
}

// ensureAnchor guarantees the chat has its welcome message. It is the only
// message the cleanup never touches, so the group-switch button is always
// reachable. recreate forces a fresh one (used by /start).
func (bs *BotService) ensureAnchor(chatID int64, recreate bool) {
	existing, ok := bs.Database.UserGroup.GetAnchorMessage(chatID)
	if ok && !recreate {
		return
	}
	if ok && recreate {
		bs.unpinMessage(chatID, existing)
		bs.deleteMessage(chatID, existing)
	}

	id := bs.sendRaw(chatID, welcomeText, anchorKeyboard(bs.ScheduleURL))
	if id == 0 {
		return
	}
	bs.pinMessage(chatID, id)

	if err := bs.Database.UserGroup.SetAnchorMessage(chatID, id); err != nil {
		Logger.Error("chat %d: failed to save anchor message: %v", chatID, err)
	}
}

func (bs *BotService) onHelp(chatID int64) {
	bs.send(chatID, "Бот показывает расписание ВТШ КФУ.\n\n"+
		"/start — главное меню\n"+
		"Отправьте номер группы (например <code>8261160</code>), чтобы выбрать её.\n"+
		"Выбранная группа запоминается, повторно вводить её не нужно —\n"+
		"сменить можно кнопкой «"+BtnChangeGroup+"».", nil)
}

// onScheduleButton skips the group prompt when the user already has one saved.
func (bs *BotService) onScheduleButton(chatID int64) {
	if g, ok := bs.savedGroup(chatID); ok {
		bs.send(chatID, fmt.Sprintf("Группа: <b>%s</b>\nВыберите период:", html.EscapeString(g.DisplayName())), scheduleKeyboard())
		return
	}
	bs.askForGroup(chatID)
}

// onPhysEd shows the whole special-medical PE timetable. Students there are
// free to attend whichever session suits them, so there is nothing to pick and
// nothing to remember - the full table is the answer.
func (bs *BotService) onPhysEd(chatID int64) {
	if !bs.PhysEd.Configured() {
		bs.send(chatID, "🏃 <b>Физра</b>\n\nРасписание физры пока не подключено.", bs.mainKeyboard())
		return
	}
	bs.send(chatID, Schedule.FormatPhysEd(bs.PhysEd.Lessons(), bs.PhysEdVenue), bs.mainKeyboard())
}

func (bs *BotService) askForGroup(chatID int64) {
	bs.ensureAnchor(chatID, false)
	bs.send(chatID, "Введите номер группы:\nПример: <code>8261160</code>", removeKeyboard())
}

// onGroupInput resolves whatever the user typed into a group and remembers it.
func (bs *BotService) onGroupInput(chatID int64, input string) {
	group, matches, found := bs.resolveGroup(input)

	if !found {
		if len(matches) > 1 {
			var b strings.Builder
			b.WriteString("Найдено несколько групп, уточните номер:\n\n")
			for _, m := range matches {
				b.WriteString("• <code>" + html.EscapeString(m.Key()) + "</code>")
				if m.Code != "" && m.Name != "" {
					b.WriteString(" — " + html.EscapeString(m.Name))
				}
				b.WriteString("\n")
			}
			bs.send(chatID, b.String(), nil)
			return
		}

		Logger.Info("chat %d: group %q not found", chatID, input)
		bs.send(chatID, fmt.Sprintf(
			"Группа <b>%s</b> не найдена.\nПроверьте номер и попробуйте ещё раз.\nПример: <code>8261160</code>",
			html.EscapeString(input)), nil)
		return
	}

	if err := bs.Database.UserGroup.SetGroup(chatID, group.Key()); err != nil {
		Logger.Error("chat %d: failed to save group %s: %v", chatID, group.Key(), err)
	} else {
		Logger.Info("chat %d: group set to %s", chatID, group.Key())
	}

	bs.send(chatID, fmt.Sprintf("Группа <b>%s</b> выбрана.\nВыберите период:",
		html.EscapeString(group.DisplayName())), scheduleKeyboard())
}

// resolveGroup maps user input to a group. It returns the match when exactly one
// group fits, or the candidate list when the input is ambiguous.
func (bs *BotService) resolveGroup(input string) (Schedule.Group, []Schedule.Group, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Schedule.Group{}, nil, false
	}

	// A bare 7-digit code is the canonical form - look it up directly.
	digits := onlyDigits(input)
	if len(digits) == groupCodeLength {
		if g, ok := bs.Schedule.Get(digits); ok {
			return g, nil, true
		}
	}

	// Otherwise accept the friendly name ("18.1-642") or a unique partial code.
	if g, ok := bs.Schedule.FindExact(input); ok {
		return g, nil, true
	}

	matches := bs.Schedule.FindMatches(input, 20)
	if len(matches) == 1 {
		return matches[0], nil, true
	}
	return Schedule.Group{}, matches, false
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// onDay sends the day card for today (offset 0) or tomorrow (offset 1),
// with the arrows that let the user page to any other date.
func (bs *BotService) onDay(chatID int64, dayOffset int) {
	g, ok := bs.requireGroup(chatID)
	if !ok {
		return
	}
	day := bs.now().AddDate(0, 0, dayOffset)
	bs.send(chatID, Schedule.FormatDayFor(g, day, bs.Calendar), dayNavKeyboard(day))
}

// showDayInPlace rewrites an existing day card for another date, so paging
// back and forth never adds a message to the chat.
func (bs *BotService) showDayInPlace(chatID int64, messageID int, day time.Time) {
	g, ok := bs.savedGroup(chatID)
	if !ok {
		bs.askForGroup(chatID)
		return
	}
	keyboard := dayNavKeyboard(day)
	if bs.editMessage(chatID, messageID, Schedule.FormatDayFor(g, day, bs.Calendar), &keyboard) {
		Logger.Info("chat %d: day card moved to %s", chatID, day.Format(dateLayout))
		return
	}
	// The original card is gone (wiped or too old) - send a fresh one instead of
	// leaving the press with no visible effect.
	bs.send(chatID, Schedule.FormatDayFor(g, day, bs.Calendar), keyboard)
}

// onWeek sends a header followed by one message per weekday. All seven are
// tracked, so the next action wipes the whole block in one go.
func (bs *BotService) onWeek(chatID int64, anchor time.Time, filterByParity bool, title string) {
	g, ok := bs.requireGroup(chatID)
	if !ok {
		return
	}

	monday := Schedule.MondayOf(anchor)
	parity := ""
	if filterByParity {
		parity = bs.Calendar.ParityFor(monday)
	}

	header := fmt.Sprintf("📋 <b>%s</b>\nГруппа: <b>%s</b>", html.EscapeString(title), html.EscapeString(g.DisplayName()))
	if filterByParity {
		header += fmt.Sprintf("\n%s — %s", monday.Format("02.01"), Schedule.ParityLabel(parity))
	}
	bs.send(chatID, header, weekKeyboard())

	for i, dayName := range Schedule.WeekDayNames {
		date := monday.AddDate(0, 0, i)
		bs.send(chatID, Schedule.FormatDay(g.DaySchedule(dayName, parity), dayName, parity, date), nil)
	}
}

// requireGroup fetches the caller's saved group, prompting for one if it is missing.
func (bs *BotService) requireGroup(chatID int64) (Schedule.Group, bool) {
	g, ok := bs.savedGroup(chatID)
	if !ok {
		bs.askForGroup(chatID)
		return Schedule.Group{}, false
	}
	return g, true
}

// savedGroup resolves the chat's persisted group key against the current schedule.
func (bs *BotService) savedGroup(chatID int64) (Schedule.Group, bool) {
	key, ok := bs.Database.UserGroup.GetGroup(chatID)
	if !ok {
		return Schedule.Group{}, false
	}
	g, ok := bs.Schedule.Get(key)
	if !ok {
		// The group disappeared from the spreadsheet (e.g. a new semester).
		Logger.Warn("chat %d: saved group %s is no longer in the schedule", chatID, key)
		return Schedule.Group{}, false
	}
	return g, true
}
