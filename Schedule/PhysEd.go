package Schedule

import (
	Logger "Bot/Logger"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// PhysEdLesson is one slot in the special-medical-group PE timetable.
// Students there pick whichever session suits them, so a lesson carries no
// group of its own - only when it runs and who teaches it.
type PhysEdLesson struct {
	Day     string // "Понедельник".."Суббота"
	Time    string // "9:40"
	Parity  string // "в", "н", or "" when the slot runs every week
	Teacher string
}

// dayAbbreviations maps what the sheet's row labels may say to a full day name.
var dayAbbreviations = map[string]string{
	"пн": "Понедельник", "понедельник": "Понедельник",
	"вт": "Вторник", "вторник": "Вторник",
	"ср": "Среда", "среда": "Среда",
	"чт": "Четверг", "четверг": "Четверг",
	"пт": "Пятница", "пятница": "Пятница",
	"сб": "Суббота", "суббота": "Суббота",
}

// PhysEdCache mirrors Cache, but for the much smaller PE sheet: it holds the
// last good snapshot in memory and refreshes it in the background.
type PhysEdCache struct {
	spreadsheetID string
	gid           string

	mu      sync.RWMutex
	lessons []PhysEdLesson
}

func NewPhysEdCache(spreadsheetID, gid string) *PhysEdCache {
	return &PhysEdCache{spreadsheetID: spreadsheetID, gid: gid}
}

// Configured reports whether a sheet was given in the config at all.
// A nil cache counts as unconfigured, so callers need no separate check.
func (c *PhysEdCache) Configured() bool {
	return c != nil && c.spreadsheetID != "" && c.gid != ""
}

func (c *PhysEdCache) Refresh() error {
	if !c.Configured() {
		return nil
	}

	rows, err := fetchSheetCSV(c.spreadsheetID, c.gid)
	if err != nil {
		Logger.Warn("physed: failed to fetch sheet: %v", err)
		return err
	}

	lessons := parsePhysEdSheet(rows)

	// Same guard as the main schedule: an empty parse must not wipe good data.
	c.mu.RLock()
	had := len(c.lessons)
	c.mu.RUnlock()
	if len(lessons) == 0 && had > 0 {
		Logger.Error("physed: parsed 0 lessons, keeping the previous snapshot")
		return errors.New("physed: refresh produced no lessons")
	}

	c.mu.Lock()
	c.lessons = lessons
	c.mu.Unlock()

	Logger.Info("physed: loaded %d lesson(s)", len(lessons))
	return nil
}

func (c *PhysEdCache) StartAutoRefresh(interval time.Duration) {
	if !c.Configured() {
		return
	}
	go func() {
		for {
			time.Sleep(interval)
			if err := c.Refresh(); err != nil {
				Logger.Error("physed: refresh error: %v", err)
			}
		}
	}()
}

// Lessons returns the current snapshot, sorted by day then time.
func (c *PhysEdCache) Lessons() []PhysEdLesson {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]PhysEdLesson, len(c.lessons))
	copy(out, c.lessons)
	return out
}

// parsePhysEdSheet reads the grid: the first row lists times, and every later
// row is labelled "<day> - <parity>" with a teacher in the cell of each slot.
func parsePhysEdSheet(rows [][]string) []PhysEdLesson {
	if len(rows) < 2 {
		return nil
	}

	times := rows[0]
	var lessons []PhysEdLesson

	for _, row := range rows[1:] {
		day, parity, ok := parsePhysEdRowLabel(cell(row, 0))
		if !ok {
			continue
		}
		for col := 1; col < len(row) && col < len(times); col++ {
			teacher := cell(row, col)
			slot := cell(times, col)
			if teacher == "" || slot == "" {
				continue
			}
			lessons = append(lessons, PhysEdLesson{
				Day:     day,
				Time:    slot,
				Parity:  parity,
				Teacher: teacher,
			})
		}
	}
	return lessons
}

// parsePhysEdRowLabel splits "пн - в" into its day and week parity. It is
// deliberately forgiving about spacing, dash style and capitalisation, since
// the sheet is maintained by hand.
func parsePhysEdRowLabel(label string) (day string, parity string, ok bool) {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return "", "", false
	}
	label = strings.NewReplacer("—", "-", "–", "-", "/", "-").Replace(label)

	parts := strings.SplitN(label, "-", 2)
	dayName, known := dayAbbreviations[strings.TrimSpace(parts[0])]
	if !known {
		return "", "", false
	}

	if len(parts) == 2 {
		switch p := strings.TrimSpace(parts[1]); {
		case strings.HasPrefix(p, "в"):
			parity = "в"
		case strings.HasPrefix(p, "н"):
			parity = "н"
		}
	}
	return dayName, parity, true
}

// physEdSlot is one printable line: a time on a day, with the teachers running
// it and the weeks it applies to.
type physEdSlot struct {
	Time    string
	Parity  string // "" means every week
	Teacher string
}

// groupPhysEdByDay collapses the raw grid into per-day lines. A slot taught by
// the same person on both weeks is printed once without a week marker, which is
// how the paper timetable reads it too.
func groupPhysEdByDay(lessons []PhysEdLesson) map[string][]physEdSlot {
	// time -> parity -> teacher, per day
	byDay := map[string]map[string]map[string]string{}
	for _, l := range lessons {
		if byDay[l.Day] == nil {
			byDay[l.Day] = map[string]map[string]string{}
		}
		if byDay[l.Day][l.Time] == nil {
			byDay[l.Day][l.Time] = map[string]string{}
		}
		byDay[l.Day][l.Time][l.Parity] = l.Teacher
	}

	out := map[string][]physEdSlot{}
	for day, slots := range byDay {
		for slotTime, byParity := range slots {
			upper, hasUpper := byParity["в"]
			lower, hasLower := byParity["н"]

			switch {
			case hasUpper && hasLower && upper == lower:
				out[day] = append(out[day], physEdSlot{Time: slotTime, Teacher: upper})
			default:
				if both, ok := byParity[""]; ok {
					out[day] = append(out[day], physEdSlot{Time: slotTime, Teacher: both})
				}
				if hasUpper {
					out[day] = append(out[day], physEdSlot{Time: slotTime, Parity: "в", Teacher: upper})
				}
				if hasLower {
					out[day] = append(out[day], physEdSlot{Time: slotTime, Parity: "н", Teacher: lower})
				}
			}
		}
		sort.Slice(out[day], func(i, j int) bool {
			if out[day][i].Time != out[day][j].Time {
				return timeSortKey(out[day][i].Time) < timeSortKey(out[day][j].Time)
			}
			return out[day][i].Parity < out[day][j].Parity
		})
	}
	return out
}
