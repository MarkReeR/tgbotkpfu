package models

type User struct {
	ID              int64
	GroupID         string
	NotifyEnabled   bool
}

type Lesson struct {
	TimeStart string
	TimeEnd   string
	Name      string
	Location  string
}

type DaySchedule struct {
	DayName   string
	Lessons   []Lesson
}
