package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/kpfu-schedule-bot/go-bot/internal/domain"
)

// SQLiteUserRepository реализует интерфейс domain.UserRepository
type SQLiteUserRepository struct {
	db     *sql.DB
	logger *log.Logger
}

// NewSQLiteUserRepository создает новый экземпляр репозитория пользователей
func NewSQLiteUserRepository(dbPath string, logger *log.Logger) (*SQLiteUserRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Включаем поддержку внешних ключей и WAL режим для лучшей производительности
	_, err = db.Exec(`
		PRAGMA foreign_keys = ON;
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}

	repo := &SQLiteUserRepository{
		db:     db,
		logger: logger,
	}

	// Создаем таблицу если не существует
	if err := repo.createTable(); err != nil {
		db.Close()
		return nil, err
	}

	return repo, nil
}

// createTable создает таблицу пользователей если она не существует
func (r *SQLiteUserRepository) createTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		group_code TEXT DEFAULT '',
		notifications_enabled INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_users_notifications ON users(notifications_enabled);
	`
	_, err := r.db.Exec(query)
	return err
}

// GetOrCreate получает или создает пользователя
func (r *SQLiteUserRepository) GetOrCreate(ctx context.Context, userID int64) (*domain.User, error) {
	// Пробуем получить существующего пользователя
	user, err := r.get(ctx, userID)
	if err == nil {
		return user, nil
	}

	// Если пользователь не найден, создаем нового
	if err == sql.ErrNoRows {
		return r.create(ctx, userID)
	}

	return nil, err
}

// get получает пользователя из БД
func (r *SQLiteUserRepository) get(ctx context.Context, userID int64) (*domain.User, error) {
	query := `
	SELECT id, group_code, notifications_enabled, created_at, updated_at
	FROM users
	WHERE id = ?
	`

	row := r.db.QueryRowContext(ctx, query, userID)
	
	var (
		id       int64
		group    string
		enabled  int
		createdAt time.Time
		updatedAt time.Time
	)

	err := row.Scan(&id, &group, &enabled, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID:                   id,
		Group:                group,
		NotificationsEnabled: enabled == 1,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}, nil
}

// create создает нового пользователя
func (r *SQLiteUserRepository) create(ctx context.Context, userID int64) (*domain.User, error) {
	query := `
	INSERT INTO users (id, group_code, notifications_enabled, created_at, updated_at)
	VALUES (?, '', 0, ?, ?)
	RETURNING id, group_code, notifications_enabled, created_at, updated_at
	`

	now := time.Now()
	var (
		id       int64
		group    string
		enabled  int
		createdAt time.Time
		updatedAt time.Time
	)

	err := r.db.QueryRowContext(ctx, query, userID, now, now).Scan(
		&id, &group, &enabled, &createdAt, &updatedAt,
	)
	if err != nil {
		// Fallback для старых версий SQLite без RETURNING
		if err.Error() != "near \"RETURNING\": syntax error" {
			return nil, err
		}
		
		// Альтернативный вариант без RETURNING
		_, err = r.db.ExecContext(ctx, 
			`INSERT OR IGNORE INTO users (id, group_code, notifications_enabled, created_at, updated_at) VALUES (?, '', 0, ?, ?)`,
			userID, now, now,
		)
		if err != nil {
			return nil, err
		}
		
		return r.get(ctx, userID)
	}

	return &domain.User{
		ID:                   id,
		Group:                group,
		NotificationsEnabled: enabled == 1,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}, nil
}

// Update обновляет данные пользователя
func (r *SQLiteUserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
	UPDATE users 
	SET group_code = ?, notifications_enabled = ?, updated_at = ?
	WHERE id = ?
	`

	enabled := 0
	if user.NotificationsEnabled {
		enabled = 1
	}

	_, err := r.db.ExecContext(ctx, query, user.Group, enabled, time.Now(), user.ID)
	return err
}

// SetNotifications устанавливает статус уведомлений
func (r *SQLiteUserRepository) SetNotifications(ctx context.Context, userID int64, enabled bool) error {
	query := `
	UPDATE users 
	SET notifications_enabled = ?, updated_at = ?
	WHERE id = ?
	`

	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	_, err := r.db.ExecContext(ctx, query, enabledInt, time.Now(), userID)
	return err
}

// GetWithNotifications получает всех пользователей с включенными уведомлениями
func (r *SQLiteUserRepository) GetWithNotifications(ctx context.Context) ([]*domain.User, error) {
	query := `
	SELECT id, group_code, notifications_enabled, created_at, updated_at
	FROM users
	WHERE notifications_enabled = 1
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var (
			id       int64
			group    string
			enabled  int
			createdAt time.Time
			updatedAt time.Time
		)

		if err := rows.Scan(&id, &group, &enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		users = append(users, &domain.User{
			ID:                   id,
			Group:                group,
			NotificationsEnabled: enabled == 1,
			CreatedAt:            createdAt,
			UpdatedAt:            updatedAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// Close закрывает соединение с БД
func (r *SQLiteUserRepository) Close() error {
	return r.db.Close()
}
