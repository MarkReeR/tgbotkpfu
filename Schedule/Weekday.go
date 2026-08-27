package Schedule

import "time"

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

// ParityFor returns "в" (числитель, upper week) for odd ISO week numbers and
// "н" (знаменатель, lower week) for even ones.
func ParityFor(t time.Time) string {
	_, week := t.ISOWeek()
	if week%2 == 1 {
		return "в"
	}
	return "н"
}

// ParityLabel returns a human-readable description of a "в"/"н" parity code.
func ParityLabel(parity string) string {
	switch parity {
	case "в":
		return "верхняя неделя"
	case "н":
		return "нижняя неделя"
	default:
		return parity
	}
}

// PairDuration is how long one class runs. The spreadsheet only lists start
// times (8:00, 9:40, 11:50, ...) and every slot is a standard 90-minute pair,
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
