package schedule

import (
	"fmt"
	"strings"
	"time"
)

type Lesson struct {
	TimeStart string
	TimeEnd   string
	Name      string
	Location  string
}

type DaySchedule struct {
	DayName string
	Lessons []Lesson
}

var lessonTimes = map[int][2]string{
	1: {"8:00", "9:30"},
	2: {"9:45", "11:15"},
	3: {"11:30", "13:00"},
	4: {"13:15", "14:45"},
	5: {"15:00", "16:30"},
	6: {"16:45", "18:15"},
	7: {"18:30", "20:00"},
}

func GetScheduleForDay(day time.Weekday, groupID string) ([]DaySchedule, error) {
	if day == time.Sunday {
		return []DaySchedule{}, nil
	}

	dayNames := map[time.Weekday]string{
		time.Monday:    "Понедельник",
		time.Tuesday:   "Вторник",
		time.Wednesday: "Среда",
		time.Thursday:  "Четверг",
		time.Friday:    "Пятница",
		time.Saturday:  "Суббота",
	}

	dayName := dayNames[day]

	lessons := generateMockLessons(groupID, day)

	return []DaySchedule{
		{
			DayName: dayName,
			Lessons: lessons,
		},
	}, nil
}

func generateMockLessons(groupID string, day time.Weekday) []Lesson {
	var lessons []Lesson

	lessonCount := 4
	if day == time.Saturday {
		lessonCount = 2
	}

	for i := 1; i <= lessonCount; i++ {
		times := lessonTimes[i]
		lessons = append(lessons, Lesson{
			TimeStart: times[0],
			TimeEnd:   times[1],
			Name:      fmt.Sprintf("Предмет %d (%s)", i, groupID),
			Location:  fmt.Sprintf("Ауд. %d", 100+i),
		})
	}

	return lessons
}

func FormatSchedule(days []DaySchedule) string {
	var result strings.Builder

	for _, day := range days {
		result.WriteString(fmt.Sprintf("%s [в]\n", day.DayName))
		result.WriteString(strings.Repeat("—", 30))
		result.WriteString("\n")

		if len(day.Lessons) == 0 {
			result.WriteString("Нет пар\n")
		} else {
			for _, lesson := range day.Lessons {
				result.WriteString(fmt.Sprintf("⏰ %s - %s\n", lesson.TimeStart, lesson.TimeEnd))
				result.WriteString(fmt.Sprintf("📚 %s\n", lesson.Name))
				result.WriteString(fmt.Sprintf("📍 %s\n\n", lesson.Location))
			}
		}
		result.WriteString("\n")
	}

	return result.String()
}
