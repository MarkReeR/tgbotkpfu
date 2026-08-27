package Schedule

import (
	"html"
	"strings"
)

// FormatPhysEd renders the whole special-medical PE timetable as one text
// message. It is deliberately plain text rather than a rendered image: students
// read it on weak connections, so it has to arrive instantly.
//
// location may be empty, in which case the venue line is left out.
func FormatPhysEd(lessons []PhysEdLesson, location string) string {
	var b strings.Builder
	b.WriteString("🏃 <b>Физра — спецмед группы</b>")
	if location != "" {
		b.WriteString("\n" + html.EscapeString(location))
	}
	b.WriteString("\n" + separator)

	if len(lessons) == 0 {
		b.WriteString("\nРасписание пока не внесено.")
		return b.String()
	}

	byDay := groupPhysEdByDay(lessons)

	printed := 0
	for _, day := range WeekDayNames {
		slots := byDay[day]
		if len(slots) == 0 {
			continue
		}
		if printed > 0 {
			b.WriteString("\n")
		}
		b.WriteString("\n<b>" + html.EscapeString(day) + "</b>")
		for _, s := range slots {
			b.WriteString("\n" + html.EscapeString(TimeRange(s.Time)))
			if s.Parity != "" {
				b.WriteString(" [" + html.EscapeString(s.Parity) + "]")
			}
			if s.Teacher != "" {
				b.WriteString(" · " + html.EscapeString(s.Teacher))
			}
		}
		printed++
	}

	if printed == 0 {
		b.WriteString("\nРасписание пока не внесено.")
	}
	return b.String()
}
