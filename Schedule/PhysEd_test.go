package Schedule

import (
	"strings"
	"testing"
)

func TestParsePhysEdRowLabel(t *testing.T) {
	cases := []struct {
		in     string
		day    string
		parity string
		ok     bool
	}{
		{"пн - в", "Понедельник", "в", true},
		{"пн-н", "Понедельник", "н", true},
		{"  ВТ — В  ", "Вторник", "в", true},
		{"суббота - н", "Суббота", "н", true},
		{"сб", "Суббота", "", true}, // no parity means every week
		{"День", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		day, parity, ok := parsePhysEdRowLabel(c.in)
		if day != c.day || parity != c.parity || ok != c.ok {
			t.Errorf("parsePhysEdRowLabel(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, day, parity, ok, c.day, c.parity, c.ok)
		}
	}
}

// The grid as the maintained sheet lays it out.
var samplePhysEdSheet = [][]string{
	{"День", "8:00", "9:40", "11:50", "13:30", "15:40"},
	{"пн - в", "", "Перепелкин В.В.", "", "", "Айдаров Р.А."},
	{"пн - н", "", "Перепелкин В.В.", "", "", ""},
	{"вт - в", "", "", "Нихорошкина А.В.", "", ""},
	{"вт - н", "", "", "Нихорошкина А.В.", "", ""},
	{"пт - в", "Снесарев С.А.", "", "", "", ""},
	{"пт - н", "Галлямова О.Н.", "", "", "", ""},
}

func TestParsePhysEdSheet(t *testing.T) {
	// 2 (пн-в) + 1 (пн-н) + 1 (вт-в) + 1 (вт-н) + 1 (пт-в) + 1 (пт-н)
	lessons := parsePhysEdSheet(samplePhysEdSheet)
	if len(lessons) != 7 {
		t.Fatalf("parsed %d lessons, want 7: %+v", len(lessons), lessons)
	}

	// Empty cells must not become lessons.
	for _, l := range lessons {
		if l.Teacher == "" || l.Time == "" || l.Day == "" {
			t.Errorf("incomplete lesson parsed: %+v", l)
		}
	}
}

func TestGroupPhysEdCollapsesIdenticalWeeks(t *testing.T) {
	byDay := groupPhysEdByDay(parsePhysEdSheet(samplePhysEdSheet))

	// Same teacher on both weeks collapses into one unmarked line...
	mon := byDay["Понедельник"]
	if len(mon) != 2 {
		t.Fatalf("Понедельник has %d slots, want 2: %+v", len(mon), mon)
	}
	if mon[0].Time != "9:40" || mon[0].Parity != "" || mon[0].Teacher != "Перепелкин В.В." {
		t.Errorf("9:40 should collapse to every-week, got %+v", mon[0])
	}
	// ...while a slot present on one week only keeps its marker.
	if mon[1].Time != "15:40" || mon[1].Parity != "в" {
		t.Errorf("15:40 should stay marked [в], got %+v", mon[1])
	}

	// Different teachers on the two weeks stay as two separate marked lines.
	fri := byDay["Пятница"]
	if len(fri) != 2 {
		t.Fatalf("Пятница has %d slots, want 2: %+v", len(fri), fri)
	}
	if fri[0].Parity != "в" || fri[1].Parity != "н" {
		t.Errorf("both weeks should be marked separately: %+v", fri)
	}
}

func TestFormatPhysEd(t *testing.T) {
	got := FormatPhysEd(parsePhysEdSheet(samplePhysEdSheet), "Спорткомплекс (ВУЗ)")

	for _, want := range []string{
		"Физра — спецмед группы",
		"Спорткомплекс (ВУЗ)",
		"<b>Понедельник</b>",
		"09:40 - 11:10 · Перепелкин В.В.",
		"15:40 - 17:10 [в] · Айдаров Р.А.",
		"<b>Пятница</b>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// Days with nothing scheduled are skipped rather than printed empty.
	for _, absent := range []string{"Среда", "Четверг", "Суббота"} {
		if strings.Contains(got, absent) {
			t.Errorf("empty day %q should be omitted:\n%s", absent, got)
		}
	}
}

func TestFormatPhysEdEmpty(t *testing.T) {
	got := FormatPhysEd(nil, "Спорткомплекс (ВУЗ)")
	if !strings.Contains(got, "Расписание пока не внесено") {
		t.Errorf("unexpected empty output:\n%s", got)
	}
}
