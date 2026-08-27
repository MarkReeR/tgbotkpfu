package Database

import (
	Repository "Bot/Database/Repository"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	UserGroup Repository.UserGroup
}

// InitDatabase opens the database and applies migrations. It returns an error
// rather than panicking so the caller can report it and shut down cleanly.
func InitDatabase(dialector gorm.Dialector) (Database, error) {
	db, err := gorm.Open(dialector, &gorm.Config{
		// GORM's default logger prints every "record not found" as an error,
		// which is a normal outcome here (a user without a saved group).
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return Database{}, fmt.Errorf("подключение к базе: %w", err)
	}

	userGroup, err := Repository.NewUserGroupRepository(db).Init()
	if err != nil {
		return Database{}, fmt.Errorf("миграция базы: %w", err)
	}

	return Database{UserGroup: userGroup}, nil
}
