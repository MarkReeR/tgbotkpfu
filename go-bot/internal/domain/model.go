package domain

import "time"

// Lesson представляет одно занятие
type Lesson struct {
	Group     string `json:"group"`
	Day       string `json:"day"`
	Time      string `json:"time"`
	WeekType  string `json:"week_type"` // "в" или "н"
	Subject   string `json:"subject"`
	Building  string `json:"building"`
	Room1     string `json:"room1"`
	Room2     string `json:"room2"`
	Type      string `json:"type"`
	Teacher   string `json:"teacher"`
	// StartTime и EndTime для отображения интервала
	StartTime time.Time
	EndTime   time.Time
}

// ScheduleCacheItem хранит кэшированное расписание пользователя
type ScheduleCacheItem struct {
	Group   string
	Lessons []Lesson
}

// GroupIndexEntry запись в индексе групп
type GroupIndexEntry struct {
	GID       int
	GroupCode string
}

// User представляет пользователя бота
type User struct {
	ID                    int64
	Group                 string
	NotificationsEnabled  bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
