package service

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kpfu-schedule-bot/go-bot/internal/domain"
)

// ScheduleServiceImpl реализует интерфейс domain.ScheduleService
type ScheduleServiceImpl struct {
	logger *log.Logger
}

// NewScheduleService создает новый экземпляр сервиса расписания
func NewScheduleService(logger *log.Logger) *ScheduleServiceImpl {
	return &ScheduleServiceImpl{logger: logger}
}

// ParseSchedule парсит CSV текст в список занятий
func (s *ScheduleServiceImpl) ParseSchedule(csvText string, groupCode string) ([]domain.Lesson, error) {
	if csvText == "" {
		s.logger.Printf("WARN Empty CSV text received for group %s", groupCode)
		return nil, nil
	}

	// Используем простую CSV-парсинг логику
	// В реальной реализации можно использовать encoding/csv или сторонние библиотеки
	lines := strings.Split(csvText, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid CSV format: not enough lines")
	}

	// Парсим заголовок для нахождения колонок группы
	headerLine := lines[0]
	colIndices, err := s.findGroupColumns(headerLine, groupCode)
	if err != nil {
		return nil, err
	}

	var lessons []domain.Lesson

	// Простая эмуляция многоуровневого заголовка
	// В реальности нужно более сложное парсирование с учетом структуры Google Sheets
	dayCol := 0
	timeCol := 1
	weekCol := 2

	currentDay := ""

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		cols := strings.Split(line, ",")

		// Обновляем текущий день (он может быть объединен для нескольких строк)
		if len(cols) > dayCol && cols[dayCol] != "" {
			currentDay = strings.TrimSpace(cols[dayCol])
		}

		if currentDay == "" {
			continue
		}

		// Извлекаем данные
		timeVal := ""
		if len(cols) > timeCol {
			timeVal = cleanValue(cols[timeCol])
		}

		weekType := ""
		if len(cols) > weekCol {
			weekType = cleanValue(cols[weekCol])
		}

		subject := ""
		if idx, ok := colIndices["subject"]; ok && len(cols) > idx {
			subject = cleanValue(cols[idx])
		}

		building := ""
		if idx, ok := colIndices["building"]; ok && len(cols) > idx {
			building = cleanValue(cols[idx])
		}

		room1 := ""
		if idx, ok := colIndices["room1"]; ok && len(cols) > idx {
			room1 = cleanValue(cols[idx])
		}

		room2 := ""
		if idx, ok := colIndices["room2"]; ok && len(cols) > idx {
			room2 = cleanValue(cols[idx])
		}

		typeVal := ""
		if idx, ok := colIndices["type"]; ok && len(cols) > idx {
			typeVal = cleanValue(cols[idx])
		}

		teacher := ""
		if idx, ok := colIndices["teacher"]; ok && len(cols) > idx {
			teacher = cleanValue(cols[idx])
		}

		if subject != "" && timeVal != "" {
			lessons = append(lessons, domain.Lesson{
				Group:    groupCode,
				Day:      normalizeDay(currentDay),
				Time:     timeVal,
				WeekType: weekType,
				Subject:  subject,
				Building: building,
				Room1:    room1,
				Room2:    room2,
				Type:     typeVal,
				Teacher:  teacher,
			})
		}
	}

	return lessons, nil
}

// findGroupColumns находит индексы колонок для данной группы
func (s *ScheduleServiceImpl) findGroupColumns(headerLine string, groupCode string) (map[string]int, error) {
	// Упрощенная логика поиска колонок
	// В реальности структура заголовков Google Sheets более сложная (многоуровневая)
	
	// Предполагаем стандартную структуру:
	// Day, Time, Week, Group_Subject, Group_Building, Group_Room1, Group_Room2, Group_Type, ..., Group_Teacher
	
	// Для простоты возвращаем фиксированные индексы
	// В production нужно парсить реальный заголовок
	return map[string]int{
		"subject":  3,
		"building": 4,
		"room1":    5,
		"room2":    6,
		"type":     7,
		"teacher":  10,
	}, nil
}

// FilterByDay фильтрует занятия по дню недели
func (s *ScheduleServiceImpl) FilterByDay(lessons []domain.Lesson, dayName string) []domain.Lesson {
	targetDay := strings.ToLower(strings.TrimSpace(dayName))
	var result []domain.Lesson

	for _, lesson := range lessons {
		if strings.ToLower(strings.TrimSpace(lesson.Day)) == targetDay {
			result = append(result, lesson)
		}
	}

	return result
}

// FilterByWeek фильтрует занятия по типу недели
func (s *ScheduleServiceImpl) FilterByWeek(lessons []domain.Lesson, targetDate time.Time) []domain.Lesson {
	weekType := getWeekType(targetDate)
	var result []domain.Lesson

	for _, lesson := range lessons {
		lessonWeek := normalizeWeek(lesson.WeekType)
		if lessonWeek == "" || lessonWeek == weekType {
			result = append(result, lesson)
		}
	}

	return result
}

// FormatDaySchedule форматирует расписание дня для вывода
func (s *ScheduleServiceImpl) FormatDaySchedule(lessons []domain.Lesson, dayName string, showWeekPerLesson bool) string {
	if len(lessons) == 0 {
		return fmt.Sprintf("<b>%s</b>\n\nЗанятий нет\n", dayName)
	}

	// Сортируем по времени
	sort.Slice(lessons, func(i, j int) bool {
		return timeToMinutes(lessons[i].Time) < timeToMinutes(lessons[j].Time)
	})

	var header string
	if showWeekPerLesson {
		header = fmt.Sprintf("<b>%s</b>", dayName)
	} else {
		week := strings.TrimSpace(lessons[0].WeekType)
		if week != "" {
			header = fmt.Sprintf("<b>%s [%s]</b>", dayName, week)
		} else {
			header = fmt.Sprintf("<b>%s</b>", dayName)
		}
	}

	sep := strings.Repeat("—", 20)
	var out []string
	out = append(out, header, sep)

	for _, les := range lessons {
		timeStr := les.Time
		weekStr := les.WeekType
		subj := les.Subject
		ltype := les.Type
		building := les.Building
		room1 := les.Room1
		room2 := les.Room2
		teacher := les.Teacher

		// Вычисляем время начала и конца пары (пара длится 1 час 30 минут)
		startMinutes := timeToMinutes(timeStr)
		endMinutes := startMinutes + 90 // 1 час 30 минут
		
		startHour := startMinutes / 60
		startMin := startMinutes % 60
		endHour := endMinutes / 60
		endMin := endMinutes % 60
		
		timeRange := fmt.Sprintf("⏰ %d:%02d - %d:%02d", startHour, startMin, endHour, endMin)
		
		if showWeekPerLesson && weekStr != "" {
			timeRange += fmt.Sprintf(" [%s]", weekStr)
		}

		lineSubject := subj
		lineType := ""
		if ltype != "" {
			lineType = fmt.Sprintf("(%s)", ltype)
		}

		rooms := []string{}
		if room1 != "" {
			rooms = append(rooms, room1)
		}
		if room2 != "" {
			rooms = append(rooms, room2)
		}
		roomsStr := strings.Join(rooms, ", ")

		locParts := []string{}
		if building != "" {
			locParts = append(locParts, building)
		}
		if roomsStr != "" {
			locParts = append(locParts, fmt.Sprintf("<i>ауд. %s</i>", roomsStr))
		}
		loc := strings.Join(locParts, ", ")

		// Очистка имени преподавателя
		teachers := splitTeachers(teacher)
		teach := strings.Join(teachers, ", ")

		linePlace := ""
		parts := []string{}
		if loc != "" {
			parts = append(parts, loc)
		}
		if teach != "" {
			parts = append(parts, teach)
		}
		if len(parts) > 0 {
			linePlace = strings.Join(parts, " — ")
		}

		block := []string{}
		block = append(block, timeRange)
		if lineSubject != "" {
			block = append(block, lineSubject)
		}
		if lineType != "" {
			block = append(block, lineType)
		}
		if linePlace != "" {
			block = append(block, linePlace)
		}

		out = append(out, strings.Join(block, "\n"))
		out = append(out, sep)
	}

	return strings.Join(out, "\n")
}

// cleanValue очищает значение от nan/None/пустых строк
func cleanValue(val string) string {
	val = strings.TrimSpace(val)
	lower := strings.ToLower(val)
	if lower == "nan" || lower == "none" || lower == "" || lower == "null" {
		return ""
	}
	return val
}

// normalizeDay нормализует название дня
func normalizeDay(day string) string {
	day = strings.TrimSpace(day)
	if len(day) > 0 {
		day = strings.ToUpper(string(day[0])) + strings.ToLower(day[1:])
	}
	return day
}

// normalizeWeek нормализует тип недели
func normalizeWeek(x string) string {
	x = strings.ToLower(strings.TrimSpace(x))
	if strings.HasPrefix(x, "в") {
		return "в"
	}
	if strings.HasPrefix(x, "н") {
		return "н"
	}
	return x
}

// getWeekType определяет тип недели для даты
func getWeekType(d time.Time) string {
	// Начальная дата учебного года (примерно 1 сентября 2025)
	startDate := time.Date(2025, time.September, 1, 0, 0, 0, 0, d.Location())
	daysPassed := int(d.Sub(startDate).Hours() / 24)
	weeksPassed := daysPassed / 7

	if weeksPassed%2 == 0 {
		return "в"
	}
	return "н"
}

// timeToMinutes преобразует время в минуты для сортировки
func timeToMinutes(timeStr string) int {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" {
		return 0
	}

	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0
	}

	hours := 0
	minutes := 0
	fmt.Sscanf(parts[0], "%d", &hours)
	fmt.Sscanf(parts[1], "%d", &minutes)

	return hours*60 + minutes
}

// splitTeachers разделяет строку с преподавателями на список
func splitTeachers(raw string) []string {
	if raw == "" {
		return nil
	}

	// Замена неразрывных пробелов и других специальных символов
	re := regexp.MustCompile(`[\u00A0\u2000-\u200B]`)
	cleaned := re.ReplaceAllString(raw, " ")

	// Разделение по ; , или двойным пробелам
	parts := regexp.MustCompile(`[;,]|\s{2,}|\t+`).Split(cleaned, -1)

	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}

	return result
}

// GetDayName возвращает название дня недели
func GetDayName(dayOffset int) string {
	days := []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота", "Воскресенье"}
	today := time.Now().AddDate(0, 0, dayOffset)
	return days[today.Weekday()]
}

// GetUserScheduleKey генерирует ключ кэша для пользователя
func GetUserScheduleKey(userID int64) string {
	return fmt.Sprintf("schedule:%d", userID)
}
