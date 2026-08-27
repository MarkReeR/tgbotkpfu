package Bot

import (
	"time"

	tgBotAPI "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Reply-keyboard captions. They double as the "commands" the user sends,
// because a reply keyboard delivers the button text as an ordinary message.
const (
	// Main menu. One icon per entry, so the top level reads at a glance;
	// the submenus below stay plain on purpose.
	BtnSchedule    = "📅 Расписание"
	BtnPhysEd      = "🏃 Физра"
	BtnExams       = "📝 Экзамены"
	BtnChangeGroup = "🔄 Смена группы"

	// Schedule submenu
	BtnToday    = "Сегодня"
	BtnTomorrow = "Завтра"
	BtnWeek     = "Вся неделя"

	// Week submenu
	BtnCurrentWeek = "Текущая неделя"
	BtnNextWeek    = "Следующая неделя"
	BtnFullWeek    = "Вся без фильтров"

	// Two distinct captions instead of one shared "Назад", so a press is
	// unambiguous without tracking which menu the user is currently in -
	// including when they tap a keyboard left over from an earlier message.
	BtnBack     = "Назад"        // week submenu -> schedule submenu
	BtnMainMenu = "Главное меню" // schedule submenu -> main menu
)

// Inline callback data. Day navigation carries the date it should move from,
// so paging stays correct without keeping per-user state on the server.
const (
	CallbackChangeGroup = "changegroup"
	CallbackMainMenu    = "mainmenu"
	CallbackDayPrefix   = "day:" // followed by a date in dateLayout
)

const dateLayout = "2006-01-02"

// mainKeyboard is the top-level menu, one button per row. The exams button is
// only present while the session is on, per the ShowExams config flag.
func mainKeyboard(showExams bool) tgBotAPI.ReplyKeyboardMarkup {
	rows := [][]tgBotAPI.KeyboardButton{
		tgBotAPI.NewKeyboardButtonRow(tgBotAPI.NewKeyboardButton(BtnSchedule)),
		tgBotAPI.NewKeyboardButtonRow(tgBotAPI.NewKeyboardButton(BtnPhysEd)),
	}
	if showExams {
		rows = append(rows, tgBotAPI.NewKeyboardButtonRow(tgBotAPI.NewKeyboardButton(BtnExams)))
	}
	rows = append(rows, tgBotAPI.NewKeyboardButtonRow(tgBotAPI.NewKeyboardButton(BtnChangeGroup)))

	kb := tgBotAPI.NewReplyKeyboard(rows...)
	kb.ResizeKeyboard = true
	return kb
}

// scheduleKeyboard is the period picker behind "Расписание".
func scheduleKeyboard() tgBotAPI.ReplyKeyboardMarkup {
	kb := tgBotAPI.NewReplyKeyboard(
		tgBotAPI.NewKeyboardButtonRow(
			tgBotAPI.NewKeyboardButton(BtnToday),
			tgBotAPI.NewKeyboardButton(BtnTomorrow),
			tgBotAPI.NewKeyboardButton(BtnWeek),
		),
		tgBotAPI.NewKeyboardButtonRow(tgBotAPI.NewKeyboardButton(BtnMainMenu)),
	)
	kb.ResizeKeyboard = true
	return kb
}

// weekKeyboard is the filter picker behind "Вся неделя".
func weekKeyboard() tgBotAPI.ReplyKeyboardMarkup {
	kb := tgBotAPI.NewReplyKeyboard(
		tgBotAPI.NewKeyboardButtonRow(
			tgBotAPI.NewKeyboardButton(BtnCurrentWeek),
			tgBotAPI.NewKeyboardButton(BtnNextWeek),
			tgBotAPI.NewKeyboardButton(BtnFullWeek),
		),
		tgBotAPI.NewKeyboardButtonRow(tgBotAPI.NewKeyboardButton(BtnBack)),
	)
	kb.ResizeKeyboard = true
	return kb
}

func removeKeyboard() tgBotAPI.ReplyKeyboardRemove {
	return tgBotAPI.NewRemoveKeyboard(false)
}

// anchorKeyboard is attached to the pinned welcome message, which is the one
// message that is never wiped - so group switching is always one tap away.
func anchorKeyboard() tgBotAPI.InlineKeyboardMarkup {
	return tgBotAPI.NewInlineKeyboardMarkup(
		tgBotAPI.NewInlineKeyboardRow(
			tgBotAPI.NewInlineKeyboardButtonData("🔄 Сменить группу", CallbackChangeGroup),
		),
	)
}

// dayNavKeyboard puts "< ☰ >" under a day card: the arrows page one day at a
// time by editing the same message in place, and the middle button drops the
// card and brings the main menu back.
func dayNavKeyboard(day time.Time) tgBotAPI.InlineKeyboardMarkup {
	prev := day.AddDate(0, 0, -1).Format(dateLayout)
	next := day.AddDate(0, 0, 1).Format(dateLayout)
	return tgBotAPI.NewInlineKeyboardMarkup(
		tgBotAPI.NewInlineKeyboardRow(
			tgBotAPI.NewInlineKeyboardButtonData("<", CallbackDayPrefix+prev),
			tgBotAPI.NewInlineKeyboardButtonData("☰", CallbackMainMenu),
			tgBotAPI.NewInlineKeyboardButtonData(">", CallbackDayPrefix+next),
		),
	)
}
