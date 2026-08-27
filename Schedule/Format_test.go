package Schedule

import (
	"strings"
	"testing"
	"time"
)

func TestTimeRange(t *testing.T) {
	cases := map[string]string{
		"8:00":  "08:00 - 09:30",
		"9:40":  "09:40 - 11:10",
		"11:50": "11:50 - 13:20",
		"13:30": "13:30 - 15:00",
		"15:40": "15:40 - 17:10",
		"17:20": "17:20 - 18:50",
		"19:00": "19:00 - 20:30",
		"":      "",
		"мусор": "мусор", // unparseable input is passed through untouched
	}
	for in, want := range cases {
		if got := TimeRange(in); got != want {
			t.Errorf("TimeRange(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatDayHeaderAndBody(t *testing.T) {
	date := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC) // Thursday
	lessons := []Lesson{{
		Day: "Четверг", Time: "9:40", Parity: "в",
		Subject: "Информатика", Kind: "лекция",
		Building: "УЛК-4", Room1: "235", Teacher: "Якупова Г.А.",
	}}

	got := FormatDay(lessons, "Четверг", "в", date)

	if !strings.Contains(got, "<b>Четверг [в]</b> · 27.08.2026") {
		t.Errorf("unexpected header in:\n%s", got)
	}
	// Time range, kind and subject.
	if !strings.Contains(got, "<b>09:40 - 11:10</b> · лекция") {
		t.Errorf("missing time/kind line in:\n%s", got)
	}
	if !strings.Contains(got, "Информатика") {
		t.Errorf("missing subject in:\n%s", got)
	}
	if !strings.Contains(got, "УЛК-4, <i>ауд. 235</i> · Якупова Г.А.") {
		t.Errorf("missing location line in:\n%s", got)
	}
	// The heavy per-lesson rules are gone: exactly one separator, under the header.
	if n := strings.Count(got, separator); n != 1 {
		t.Errorf("expected 1 separator, got %d in:\n%s", n, got)
	}
}

func TestFormatDayEmptyAndSunday(t *testing.T) {
	date := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC) // Saturday
	empty := FormatDay(nil, "Суббота", "н", date)
	if !strings.Contains(empty, "Занятий нет") || !strings.Contains(empty, "29.08.2026") {
		t.Errorf("unexpected empty-day card:\n%s", empty)
	}

	sunday := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	got := FormatDayFor(Group{Code: "8261160"}, sunday)
	if !strings.Contains(got, "Воскресенье") || !strings.Contains(got, "воскресенье)") {
		t.Errorf("unexpected sunday card:\n%s", got)
	}
}

func TestFormatDayUnfilteredMarksEachLesson(t *testing.T) {
	date := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	lessons := []Lesson{
		{Time: "8:00", Parity: "в", Subject: "Физика"},
		{Time: "8:00", Parity: "н", Subject: "Химия"},
	}
	got := FormatDay(lessons, "Четверг", "", date)

	if strings.Contains(got, "Четверг [") {
		t.Errorf("header should carry no parity when unfiltered:\n%s", got)
	}
	if !strings.Contains(got, "[в]") || !strings.Contains(got, "[н]") {
		t.Errorf("each lesson should carry its own parity:\n%s", got)
	}
}
