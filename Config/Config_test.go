package Config

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleINI = `
[runArgs]
BotKey = token-from-file
LogLevel = WARN
DatabasePath = file.db
Timezone = Europe/Moscow

[schedule]
SpreadsheetId = sheet-from-file
Sheets = 1 курс:0, 2 курс:123
RefreshMinutes = 20

[physed]
SpreadsheetId = pe-sheet
Gid = 7
Venue = Спорткомплекс

[exams]
Show = true
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.ini")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFromFile(t *testing.T) {
	cfg, err := Load(writeConfig(t, sampleINI))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.BotToken != "token-from-file" {
		t.Errorf("BotToken = %q", cfg.BotToken)
	}
	if cfg.LogLevel != "WARN" || cfg.DatabasePath != "file.db" {
		t.Errorf("unexpected run args: %+v", cfg)
	}
	if cfg.RefreshMinutes != 20 || cfg.RefreshInterval().Minutes() != 20 {
		t.Errorf("RefreshMinutes = %d", cfg.RefreshMinutes)
	}
	if len(cfg.Sheets) != 2 || cfg.Sheets[1].Name != "2 курс" || cfg.Sheets[1].Gid != "123" {
		t.Errorf("Sheets = %+v", cfg.Sheets)
	}
	if cfg.PhysEdGid != "7" || cfg.PhysEdVenue != "Спорткомплекс" {
		t.Errorf("physed = %+v", cfg)
	}
	if !cfg.ShowExams {
		t.Error("ShowExams should be true")
	}
	if cfg.Location == nil {
		t.Error("Location must never be nil")
	}
}

func TestEnvOverridesFile(t *testing.T) {
	t.Setenv("BOT_TOKEN", "token-from-env")
	t.Setenv("REFRESH_MINUTES", "5")
	t.Setenv("EXAMS_SHOW", "false")
	t.Setenv("PHYSED_GID", "99")

	cfg, err := Load(writeConfig(t, sampleINI))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.BotToken != "token-from-env" {
		t.Errorf("env should win: %q", cfg.BotToken)
	}
	if cfg.RefreshMinutes != 5 {
		t.Errorf("RefreshMinutes = %d", cfg.RefreshMinutes)
	}
	if cfg.ShowExams {
		t.Error("EXAMS_SHOW=false should win over the file")
	}
	if cfg.PhysEdGid != "99" {
		t.Errorf("PhysEdGid = %q", cfg.PhysEdGid)
	}
}

// The container case: no config file at all, everything from the environment.
func TestLoadWithoutFile(t *testing.T) {
	t.Setenv("BOT_TOKEN", "t")
	t.Setenv("SPREADSHEET_ID", "s")
	t.Setenv("SHEETS", "1 курс:0")

	cfg, err := Load(filepath.Join(t.TempDir(), "absent.ini"))
	if err != nil {
		t.Fatalf("a missing file must not be fatal when env is complete: %v", err)
	}
	if len(cfg.Sheets) != 1 || cfg.Sheets[0].Gid != "0" {
		t.Errorf("Sheets = %+v", cfg.Sheets)
	}
	if cfg.LogDir != "logs" || cfg.DatabasePath != "Database.db" {
		t.Errorf("defaults not applied: %+v", cfg)
	}
}

func TestValidationErrors(t *testing.T) {
	cases := map[string]string{
		"no token": `
[schedule]
SpreadsheetId = s
Sheets = 1 курс:0
`,
		"no spreadsheet": `
[runArgs]
BotKey = t
`,
		"no sheets": `
[runArgs]
BotKey = t
[schedule]
SpreadsheetId = s
`,
		"bad refresh": `
[runArgs]
BotKey = t
[schedule]
SpreadsheetId = s
Sheets = 1 курс:0
RefreshMinutes = 0
`,
	}
	for name, body := range cases {
		if _, err := Load(writeConfig(t, body)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestParseSheetSources(t *testing.T) {
	got := ParseSheetSources(" 1 курс:0 , 2 курс:123 ,,broken, :5, name: ")
	if len(got) != 2 {
		t.Fatalf("parsed %d sources, want 2: %+v", len(got), got)
	}
	if got[0].Name != "1 курс" || got[0].Gid != "0" {
		t.Errorf("first source = %+v", got[0])
	}
	if ParseSheetSources("") != nil {
		t.Error("empty input should yield no sources")
	}
}
