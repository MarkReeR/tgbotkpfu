package Schedule

import "time"

// Week parity markers as the spreadsheet spells them.
const (
	ParityUpper = "в" // верхняя неделя (числитель)
	ParityLower = "н" // нижняя неделя (знаменатель)
)

var dayNames = [...]string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

// DayName returns the Russian weekday name and whether the university has any
// schedule for it at all (there is none for Sunday).
func DayName(t time.Time) (string, bool) {
	wd := t.Weekday()
	if wd == time.Sunday {
		return "", false
	}
	return dayNames[int(wd)-1], true
}

// Calendar decides which week is "в" and which is "н". Parity is counted from
// the start of the semester rather than from the ISO week number, because the
// university anchors it to the first study week: the week containing the
// semester start is the upper one, and they alternate from there.
type Calendar struct {
	anchorMonday time.Time
}

// NewCalendar anchors parity to the week containing semesterStart, which is
// treated as an upper ("в") week.
func NewCalendar(semesterStart time.Time) Calendar {
	return Calendar{anchorMonday: MondayOf(semesterStart)}
}

// ParityFor returns the week marker for the calendar week t falls in.
func (c Calendar) ParityFor(t time.Time) string {
	weeks := weeksBetween(c.anchorMonday, MondayOf(t))
	// Go's % keeps the sign of the dividend, so normalise for dates that fall
	// before the semester start (the user can page back that far).
	if ((weeks%2)+2)%2 == 0 {
		return ParityUpper
	}
	return ParityLower
}

// weeksBetween counts whole weeks between two Mondays, and may be negative.
func weeksBetween(from, to time.Time) int {
	return int(dayNumber(to)-dayNumber(from)) / 7
}

// dayNumber maps a date to a day count, ignoring clock time and zone so that
// daylight-saving shifts cannot turn a week into 6 days and 23 hours.
func dayNumber(t time.Time) int64 {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400
}

// ParityLabel returns a human-readable description of a "в"/"н" parity code.
func ParityLabel(parity string) string {
	switch parity {
	case ParityUpper:
		return "верхняя неделя"
	case ParityLower:
		return "нижняя неделя"
	default:
		return parity
	}
}

// PairDuration is how long one class runs. The spreadsheet only lists start
// times (8:00, 9:40, ...) and every slot is a standard 90-minute pair,
// so the end time is derived rather than stored.
const PairDuration = 90 * time.Minute

// TimeRange turns a start time like "8:00" into "08:00 - 09:30".
// It returns the input unchanged if it cannot be parsed.
func TimeRange(start string) string {
	t, err := time.Parse("15:04", normalizeClock(start))
	if err != nil {
		return start
	}
	return t.Format("15:04") + " - " + t.Add(PairDuration).Format("15:04")
}

// normalizeClock pads a single-digit hour ("8:00" -> "08:00") so it parses.
func normalizeClock(s string) string {
	if len(s) == 4 && s[1] == ':' {
		return "0" + s
	}
	return s
}
