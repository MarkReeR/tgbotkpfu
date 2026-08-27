package Repository

import (
	Table "Bot/Database/Repository/Table"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserGroup struct {
	database *gorm.DB
}

func NewUserGroupRepository(_database *gorm.DB) UserGroup {
	return UserGroup{
		database: _database,
	}
}

// Init applies the schema migration. A failure here means the bot cannot store
// anything, so it is reported rather than swallowed.
func (r UserGroup) Init() (UserGroup, error) {
	if err := r.database.AutoMigrate(&Table.UserGroup{}); err != nil {
		return r, err
	}
	return r, nil
}

// GetGroup returns the group key the chat previously selected, if any.
func (r UserGroup) GetGroup(chatID int64) (string, bool) {
	tg, err := r.row(chatID)
	if err != nil || tg.GroupNumber == "" {
		return "", false
	}
	return tg.GroupNumber, true
}

// SetGroup remembers the chat's group without disturbing its anchor message.
func (r UserGroup) SetGroup(chatID int64, groupNumber string) error {
	return r.upsert(Table.UserGroup{ChatID: chatID, GroupNumber: groupNumber}, "group_number")
}

// GetAnchorMessage returns the chat's pinned welcome message id, if one is known.
func (r UserGroup) GetAnchorMessage(chatID int64) (int, bool) {
	tg, err := r.row(chatID)
	if err != nil || tg.AnchorMessageID == 0 {
		return 0, false
	}
	return tg.AnchorMessageID, true
}

// SetAnchorMessage remembers the chat's anchor message without touching its group.
func (r UserGroup) SetAnchorMessage(chatID int64, messageID int) error {
	return r.upsert(Table.UserGroup{ChatID: chatID, AnchorMessageID: messageID}, "anchor_message_id")
}

func (r UserGroup) row(chatID int64) (Table.UserGroup, error) {
	tg := Table.UserGroup{ChatID: chatID}
	err := r.database.Where(&Table.UserGroup{ChatID: chatID}).Take(&tg).Error
	return tg, err
}

// upsert inserts the row or updates only the named columns, so writing one
// field never blanks out the other.
func (r UserGroup) upsert(row Table.UserGroup, columns ...string) error {
	return r.database.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns(columns),
	}).Create(&row).Error
}
