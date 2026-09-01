package Schedule

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// The autumn semester starts on 1 September 2026 (a Tuesday), and that whole
// week is the upper one.
func TestParityAnchoredToSemesterStart(t *testing.T) {
	cal := NewCalendar(date(2026, time.September, 1))

	cases := []struct {
		day  time.Time
		want string
		why  string
	}{
		{date(2026, time.August, 31), ParityUpper, "понедельник той же недели, что и 1 сентября"},
		{date(2026, time.September, 1), ParityUpper, "сам день начала семестра"},
		{date(2026, time.September, 5), ParityUpper, "суббота первой недели"},
		{date(2026, time.September, 6), ParityUpper, "воскресенье относится к закончившейся неделе"},
		{date(2026, time.September, 7), ParityLower, "вторая неделя"},
		{date(2026, time.September, 8), ParityLower, "вторник второй недели"},
		{date(2026, time.September, 14), ParityUpper, "третья неделя"},
		{date(2026, time.September, 21), ParityLower, "четвёртая неделя"},
		{date(2026, time.December, 28), ParityLower, "через 17 недель, число нечётное"},
		// Paging backwards past the semester start must not break the alternation.
		{date(2026, time.August, 24), ParityLower, "неделя до начала семестра"},
		{date(2026, time.August, 17), ParityUpper, "две недели до начала"},
	}

	for _, c := range cases {
		if got := cal.ParityFor(c.day); got != c.want {
			t.Errorf("ParityFor(%s) = %q, want %q (%s)",
				c.day.Format("02.01.2006"), got, c.want, c.why)
		}
	}
}

// The anchor may be given as any day of the first week, not just the Monday.
func TestParityAnchorAcceptsAnyWeekday(t *testing.T) {
	monday := NewCalendar(date(2026, time.August, 31))
	tuesday := NewCalendar(date(2026, time.September, 1))
	saturday := NewCalendar(date(2026, time.September, 5))

	for _, day := range []time.Time{
		date(2026, time.September, 3),
		date(2026, time.September, 10),
		date(2026, time.October, 15),
	} {
		a, b, c := monday.ParityFor(day), tuesday.ParityFor(day), saturday.ParityFor(day)
		if a != b || b != c {
			t.Errorf("%s: якорь в разные дни одной недели дал %q/%q/%q",
				day.Format("02.01.2006"), a, b, c)
		}
	}
}

// Parity must not drift across a daylight-saving change in a zone that has one.
func TestParitySurvivesDaylightSaving(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("tz database unavailable")
	}
	start := time.Date(2026, time.September, 1, 0, 0, 0, 0, berlin)
	cal := NewCalendar(start)

	// Europe/Berlin moves the clock back on 25 October 2026.
	before := time.Date(2026, time.October, 19, 12, 0, 0, 0, berlin) // 7 недель -> нижняя
	after := time.Date(2026, time.October, 26, 12, 0, 0, 0, berlin)  // 8 недель -> верхняя

	if got := cal.ParityFor(before); got != ParityLower {
		t.Errorf("19.10 = %q, want %q", got, ParityLower)
	}
	if got := cal.ParityFor(after); got != ParityUpper {
		t.Errorf("26.10 = %q, want %q", got, ParityUpper)
	}
}

func TestMondayOf(t *testing.T) {
	cases := map[string]time.Time{
		"31.08.2026": date(2026, time.August, 31), // понедельник
		"01.09.2026": date(2026, time.August, 31), // вторник
		"06.09.2026": date(2026, time.August, 31), // воскресенье -> прошедшая неделя
		"07.09.2026": date(2026, time.September, 7),
	}
	for in, want := range cases {
		parsed, _ := time.Parse("02.01.2006", in)
		if got := MondayOf(parsed); !got.Equal(want) {
			t.Errorf("MondayOf(%s) = %s, want %s", in,
				got.Format("02.01.2006"), want.Format("02.01.2006"))
		}
	}
}
