package Schedule

import (
	Logger "Bot/Logger"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Cache holds the last successfully parsed snapshot of the schedule spreadsheet
// in memory and refreshes it periodically, so request handlers never block on
// a network call to Google Sheets.
//
// The snapshot is an *index: a set of hash tables built once per refresh, so a
// user's lookup never scans the group list.
type Cache struct {
	spreadsheetID string
	sources       []SheetSource

	mu  sync.RWMutex
	idx *index
}

func NewCache(spreadsheetID string, sources []SheetSource) *Cache {
	return &Cache{
		spreadsheetID: spreadsheetID,
		sources:       sources,
		idx:           newIndex(nil),
	}
}

// Refresh re-downloads every course sheet and swaps the in-memory snapshot.
// If every sheet fails to load, the previous snapshot is kept.
func (c *Cache) Refresh() error {
	byKey := map[string]Group{}
	var firstErr error
	loaded := 0

	for _, src := range c.sources {
		rows, err := fetchSheetCSV(c.spreadsheetID, src.Gid)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			Logger.Warn("schedule: failed to fetch sheet %q: %v", src.Name, err)
			continue
		}
		loaded++
		course := courseFromSheetName(src.Name)
		for _, g := range parseCourseSheet(course, rows) {
			byKey[g.Key()] = g
		}
	}

	if loaded == 0 {
		return firstErr
	}

	// Never replace a good snapshot with an empty one. A sheet whose layout
	// changed, or that came back as something other than the expected CSV, would
	// otherwise leave every user with an empty schedule until the next restart.
	if len(byKey) == 0 && c.Size() > 0 {
		Logger.Error("schedule: parsed 0 groups from %d sheet(s), keeping the previous snapshot", loaded)
		return fmt.Errorf("schedule: refresh produced no groups")
	}

	groups := make([]Group, 0, len(byKey))
	for _, g := range byKey {
		groups = append(groups, g)
	}
	next := newIndex(groups)

	c.mu.Lock()
	c.idx = next
	c.mu.Unlock()

	Logger.Info("schedule: loaded %d sheet(s), %d group(s)", loaded, next.size())
	return firstErr
}

// StartAutoRefresh refreshes the cache on a fixed interval until the process exits.
func (c *Cache) StartAutoRefresh(interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			if err := c.Refresh(); err != nil {
				Logger.Error("schedule: refresh error: %v", err)
			}
		}
	}()
}

// snapshot returns the current index. Callers must not hold it across a refresh
// boundary expecting fresh data, but the index itself is immutable and safe to
// read without further locking.
func (c *Cache) snapshot() *index {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.idx
}

// Get returns the group with the given key (numeric code, or name when the
// sheet gives no code).
func (c *Cache) Get(key string) (Group, bool) {
	return c.snapshot().get(key)
}

// FindExact returns the group whose code or name matches the query exactly.
func (c *Cache) FindExact(query string) (Group, bool) {
	return c.snapshot().findExact(query)
}

// FindMatches returns every group whose code or name contains the query.
func (c *Cache) FindMatches(query string, limit int) []Group {
	return c.snapshot().findMatches(query, limit)
}

// Size reports how many groups the current snapshot holds.
func (c *Cache) Size() int {
	return c.snapshot().size()
}

// DaySchedule returns this group's lessons for one day, filtered by week parity, sorted by time.
// Pass an empty parity to keep both weeks.
func (g Group) DaySchedule(day string, parity string) []Lesson {
	var out []Lesson
	for _, l := range g.Lessons {
		if l.Day == day && (parity == "" || l.Parity == parity) {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Time != out[j].Time {
			return timeSortKey(out[i].Time) < timeSortKey(out[j].Time)
		}
		return out[i].Parity < out[j].Parity // "в" before "н" within one slot
	})
	return out
}

// timeSortKey turns "8:00"/"13:30" into minutes-since-midnight for stable sorting.
func timeSortKey(t string) int {
	parts := strings.SplitN(t, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	return h*60 + m
}
