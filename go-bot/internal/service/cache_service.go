package service

import (
	"context"
	"sync"
	"time"

	"github.com/kpfu-schedule-bot/go-bot/internal/domain"
)

// MemoryCacheService реализует кэширование в памяти
type MemoryCacheService struct {
	data map[string]*cacheEntry
	mu   sync.RWMutex
}

type cacheEntry struct {
	item      *domain.ScheduleCacheItem
	expiresAt time.Time
}

// NewMemoryCacheService создает новый экземпляр кэш-сервиса
func NewMemoryCacheService() *MemoryCacheService {
	s := &MemoryCacheService{
		data: make(map[string]*cacheEntry),
	}

	// Запускаем фоновую задачу для очистки устаревших записей
	go s.cleanupLoop()

	return s
}

// Get получает значение из кэша
func (s *MemoryCacheService) Get(ctx context.Context, key string) (*domain.ScheduleCacheItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.data[key]
	if !exists {
		return nil, nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil, nil
	}

	return entry.item, nil
}

// Set устанавливает значение в кэш
func (s *MemoryCacheService) Set(ctx context.Context, key string, item *domain.ScheduleCacheItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// TTL = 3 дня (как в Python версии)
	s.data[key] = &cacheEntry{
		item:      item,
		expiresAt: time.Now().Add(72 * time.Hour),
	}

	return nil
}

// Delete удаляет значение из кэша
func (s *MemoryCacheService) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// cleanupLoop периодически удаляет устаревшие записи
func (s *MemoryCacheService) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup удаляет устаревшие записи
func (s *MemoryCacheService) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.data {
		if now.After(entry.expiresAt) {
			delete(s.data, key)
		}
	}
}

// UserServiceImpl реализует сервис работы с пользователями
type UserServiceImpl struct {
	cache domain.CacheService
	repo  domain.UserRepository
}

// NewUserService создает новый экземпляр сервиса пользователей
func NewUserService(cache domain.CacheService, repo domain.UserRepository) *UserServiceImpl {
	return &UserServiceImpl{cache: cache, repo: repo}
}

// GetUserSchedule получает расписание пользователя
func (s *UserServiceImpl) GetUserSchedule(ctx context.Context, userID int64) (*domain.ScheduleCacheItem, error) {
	key := GetUserScheduleKey(userID)
	return s.cache.Get(ctx, key)
}

// SetUserSchedule устанавливает расписание пользователя
func (s *UserServiceImpl) SetUserSchedule(ctx context.Context, userID int64, group string, lessons []domain.Lesson) error {
	// Сохраняем в кэш
	key := GetUserScheduleKey(userID)
	item := &domain.ScheduleCacheItem{
		Group:   group,
		Lessons: lessons,
	}
	if err := s.cache.Set(ctx, key, item); err != nil {
		return err
	}

	// Сохраняем группу в БД для персистентности
	user, err := s.repo.GetOrCreate(ctx, userID)
	if err != nil {
		return err
	}
	user.Group = group
	return s.repo.Update(ctx, user)
}

// GetUser получает данные пользователя из БД
func (s *UserServiceImpl) GetUser(ctx context.Context, userID int64) (*domain.User, error) {
	return s.repo.GetOrCreate(ctx, userID)
}

// SetNotifications устанавливает статус уведомлений пользователя
func (s *UserServiceImpl) SetNotifications(ctx context.Context, userID int64, enabled bool) error {
	return s.repo.SetNotifications(ctx, userID, enabled)
}
