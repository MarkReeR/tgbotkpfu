package Table

type UserGroup struct {
	ChatID      int64  `gorm:"primaryKey;autoIncrement:false"`
	GroupNumber string `gorm:"default:''"`
	// AnchorMessageID is the pinned welcome message that is never wiped when the
	// chat is cleaned up. Stored so it survives a restart of the bot.
	AnchorMessageID int `gorm:"default:0"`
}
