package Repository

import (
	Table "Bot/Database/Repository/Table"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestRepo(t *testing.T) UserGroup {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Table.UserGroup{}); err != nil {
		t.Fatal(err)
	}
	return NewUserGroupRepository(db)
}

func TestGroupAndAnchorDoNotOverwriteEachOther(t *testing.T) {
	r := newTestRepo(t)
	const chat int64 = 42

	if _, ok := r.GetGroup(chat); ok {
		t.Fatal("expected no group initially")
	}
	if _, ok := r.GetAnchorMessage(chat); ok {
		t.Fatal("expected no anchor initially")
	}

	if err := r.SetGroup(chat, "8261160"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetAnchorMessage(chat, 777); err != nil {
		t.Fatal(err)
	}

	// Writing the anchor must not have blanked the group.
	if got, ok := r.GetGroup(chat); !ok || got != "8261160" {
		t.Errorf("group after anchor write = %q, %v", got, ok)
	}
	if got, ok := r.GetAnchorMessage(chat); !ok || got != 777 {
		t.Errorf("anchor = %d, %v", got, ok)
	}

	// ...and updating the group must not have dropped the anchor.
	if err := r.SetGroup(chat, "18.1-512"); err != nil {
		t.Fatal(err)
	}
	if got, ok := r.GetGroup(chat); !ok || got != "18.1-512" {
		t.Errorf("group after update = %q, %v", got, ok)
	}
	if got, ok := r.GetAnchorMessage(chat); !ok || got != 777 {
		t.Errorf("anchor lost after group update: %d, %v", got, ok)
	}

	// A second chat must be independent.
	if _, ok := r.GetGroup(43); ok {
		t.Error("unrelated chat should have no group")
	}
}

func TestAnchorBeforeGroup(t *testing.T) {
	r := newTestRepo(t)
	const chat int64 = 7

	// The anchor is created on /start, before any group is chosen.
	if err := r.SetAnchorMessage(chat, 100); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.GetGroup(chat); ok {
		t.Error("anchor write should not invent a group")
	}
	if err := r.SetGroup(chat, "8261160"); err != nil {
		t.Fatal(err)
	}
	if got, ok := r.GetAnchorMessage(chat); !ok || got != 100 {
		t.Errorf("anchor = %d, %v", got, ok)
	}
}
