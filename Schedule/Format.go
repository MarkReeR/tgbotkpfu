package Schedule

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// A single light rule under the day header. Lessons below are separated by a
// blank line rather than another rule, which keeps a long day readable.
const separator = "──────────────────────"

var allDayNames = [...]string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

// WeekDayNames lists the six days the university actually schedules classes on.
var WeekDayNames = []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

// WeekdayNameAny returns the Russian name for any day, Sunday included.
func WeekdayNameAny(t time.Time) string {
	return allDayNames[int(t.Weekday())]
}

// MondayOf returns midnight of the Monday belonging to t's calendar week.
func MondayOf(t time.Time) time.Time {
	offset := int(t.Weekday())
	if offset == 0 { // Sunday belongs to the week that just ended
		offset = 7
	}
	y, m, d := t.AddDate(0, 0, 1-offset).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// formatLesson renders one lesson. showParity puts the week marker on each
// lesson instead of in the day header, which the unfiltered week view needs.
func formatLesson(l Lesson, showParity bool) string {
	var lines []string

	// First line: "09:40 - 11:10 · лекция", with the week marker when the view
	// mixes both weeks together.
	if l.Time != "" {
		timeLine := "<b>" + html.EscapeString(TimeRange(l.Time)) + "</b>"
		if showParity && l.Parity != "" {
			timeLine += " [" + html.EscapeString(l.Parity) + "]"
		}
		if l.Kind != "" {
			timeLine += " · " + html.EscapeString(l.Kind)
		}
		lines = append(lines, timeLine)
	}
	if l.Subject != "" {
		lines = append(lines, html.EscapeString(l.Subject))
	}

	// Location and teacher share the last line: "УЛК-4, ауд. 235 — Иванов И.И."
	rooms := make([]string, 0, 2)
	for _, r := range []string{l.Room1, l.Room2} {
		if r != "" {
			rooms = append(rooms, r)
		}
	}
	locParts := make([]string, 0, 2)
	if l.Building != "" {
		locParts = append(locParts, html.EscapeString(l.Building))
	}
	if len(rooms) > 0 {
		locParts = append(locParts, "<i>ауд. "+html.EscapeString(strings.Join(rooms, ", "))+"</i>")
	}
	loc := strings.Join(locParts, ", ")
	teacher := html.EscapeString(strings.TrimSpace(l.Teacher))

	switch {
	case loc != "" && teacher != "":
		lines = append(lines, loc+" · "+teacher)
	case loc != "":
		lines = append(lines, loc)
	case teacher != "":
		lines = append(lines, teacher)
	}

	return strings.Join(lines, "\n")
}

// dayHeader builds "Четверг [в] 27.08" - the weekday, the week parity and the date.
// parity may be empty, in which case the marker is left off.
func dayHeader(dayName string, parity string, date time.Time) string {
	header := "📅 <b>" + html.EscapeString(dayName)
	if parity != "" {
		header += " [" + html.EscapeString(parity) + "]"
	}
	header += "</b> · " + date.Format("02.01.2006")
	return header
}

// FormatDay renders one day as a single HTML message. Pass an empty parity to
// mark each lesson with its own instead of the whole day.
func FormatDay(lessons []Lesson, dayName string, parity string, date time.Time) string {
	header := dayHeader(dayName, parity, date)
	if len(lessons) == 0 {
		return header + "\n" + separator + "\nЗанятий нет"
	}

	blocks := make([]string, 0, len(lessons))
	for _, l := range lessons {
		blocks = append(blocks, formatLesson(l, parity == ""))
	}
	return header + "\n" + separator + "\n" + strings.Join(blocks, "\n\n")
}

// FormatDayFor renders the schedule of one group for the calendar day t falls on,
// picking the right week parity from the calendar.
func FormatDayFor(g Group, t time.Time, cal Calendar) string {
	dayName, isStudyDay := DayName(t)
	if !isStudyDay {
		return fmt.Sprintf("📅 <b>%s</b> · %s\n%s\nЗанятий нет (воскресенье)",
			html.EscapeString(WeekdayNameAny(t)), t.Format("02.01.2006"), separator)
	}
	parity := cal.ParityFor(t)
	return FormatDay(g.DaySchedule(dayName, parity), dayName, parity, t)
}
