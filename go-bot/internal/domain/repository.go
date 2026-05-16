package domain

import (
	"context"
	"time"
)

// CSVRepository определяет интерфейс для работы с CSV файлами
type CSVRepository interface {
	// DownloadAll скачивает все CSV файлы для указанных GID
	DownloadAll(ctx context.Context, gids []int) error
	// RefreshAll обновляет все CSV файлы
	RefreshAll(ctx context.Context) error
	// FindGroupSchedule находит расписание для группы в локальном кэше
	FindGroupSchedule(ctx context.Context, groupCode string) (string, error)
	// BuildIndex строит индекс групп по всем CSV файлам
	BuildIndex() error
}

// ScheduleService определяет интерфейс сервиса расписания
type ScheduleService interface {
	// ParseSchedule парсит CSV текст в список занятий
	ParseSchedule(csvText string, groupCode string) ([]Lesson, error)
	// FilterByDay фильтрует занятия по дню недели
	FilterByDay(lessons []Lesson, dayName string) []Lesson
	// FilterByWeek фильтрует занятия по типу недели
	FilterByWeek(lessons []Lesson, targetDate time.Time) []Lesson
	// FormatDaySchedule форматирует расписание дня для вывода
	FormatDaySchedule(lessons []Lesson, dayName string, showWeekPerLesson bool) string
}

// CacheService определяет интерфейс для кэширования
type CacheService interface {
	// Get получает значение из кэша
	Get(ctx context.Context, key string) (*ScheduleCacheItem, error)
	// Set устанавливает значение в кэш
	Set(ctx context.Context, key string, item *ScheduleCacheItem) error
	// Delete удаляет значение из кэша
	Delete(ctx context.Context, key string) error
}

// UserService определяет интерфейс для работы с пользователями
type UserService interface {
	// GetUserSchedule получает расписание пользователя
	GetUserSchedule(ctx context.Context, userID int64) (*ScheduleCacheItem, error)
	// SetUserSchedule устанавливает расписание пользователя
	SetUserSchedule(ctx context.Context, userID int64, group string, lessons []Lesson) error
	// GetUser получает данные пользователя из БД
	GetUser(ctx context.Context, userID int64) (*User, error)
	// SetNotifications устанавливает статус уведомлений
	SetNotifications(ctx context.Context, userID int64, enabled bool) error
}

// UserRepository определяет интерфейс для работы с пользователями в БД
type UserRepository interface {
	// GetOrCreate получает или создает пользователя
	GetOrCreate(ctx context.Context, userID int64) (*User, error)
	// Update обновляет данные пользователя
	Update(ctx context.Context, user *User) error
	// SetNotifications устанавливает статус уведомлений
	SetNotifications(ctx context.Context, userID int64, enabled bool) error
	// GetWithNotifications получает всех пользователей с включенными уведомлениями
	GetWithNotifications(ctx context.Context) ([]*User, error)
}
