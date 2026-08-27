package Schedule

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	headerRow      = 0 // row with group code/name per block
	columnLabelRow = 1 // row with "Дисциплина/Здание/..." labels, unused
	firstDataRow   = 2

	dayCol    = 0
	timeCol   = 1
	parityCol = 2

	blockStart = 3  // first column of the first group's block (0-indexed)
	blockWidth = 10 // Дисциплина, Здание, АУД.1, АУД.2, Вид, Вед.каф, Должн., Преподаватель, combined room, spacer

	offSubject    = 0
	offBuilding   = 1
	offRoom1      = 2
	offRoom2      = 3
	offKind       = 4
	offDepartment = 5
	offPosition   = 6
	offTeacher    = 7
)

// A group header is one of three shapes:
//
//	"8261160 (18.1-642)" - numeric code plus friendly name
//	"8241160"            - numeric code only
//	"18.1-512"           - friendly name only (no code at all)
//
// The digits must make up the whole token, otherwise a name like "18.1-512"
// would be mistaken for the code "18" and every such group would collide.
var (
	groupCodeAndNameRe = regexp.MustCompile(`^(\d+)\s*\((.+)\)$`)
	groupCodeOnlyRe    = regexp.MustCompile(`^(\d+)$`)
)

// parseGroupHeader splits a header cell into a numeric code and a friendly name.
// It reports false when the cell holds no group at all (blank or a stray label).
func parseGroupHeader(raw string) (code string, name string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	if m := groupCodeAndNameRe.FindStringSubmatch(raw); m != nil {
		return m[1], strings.TrimSpace(m[2]), true
	}
	if m := groupCodeOnlyRe.FindStringSubmatch(raw); m != nil {
		return m[1], "", true
	}
	// Anything else is treated as a name-only group, as long as it carries a digit
	// (guards against stray text such as a merged-cell label leaking in).
	if strings.ContainsAny(raw, "0123456789") {
		return "", raw, true
	}
	return "", "", false
}

func cell(row []string, col int) string {
	if col < 0 || col >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[col])
}

// parseCourseSheet turns the raw CSV grid of one course tab into its groups.
func parseCourseSheet(course int, rows [][]string) []Group {
	if len(rows) <= firstDataRow {
		return nil
	}

	header := rows[headerRow]

	// Discover every block's group code/name from the header row.
	groups := make(map[int]*Group) // keyed by block start column
	for col := blockStart; col < len(header); col += blockWidth {
		code, name, ok := parseGroupHeader(header[col])
		if !ok {
			continue
		}
		groups[col] = &Group{
			Code:   code,
			Name:   name,
			Course: course,
		}
	}
	if len(groups) == 0 {
		return nil
	}

	currentDay := ""
	for r := firstDataRow; r < len(rows); r++ {
		row := rows[r]

		if d := cell(row, dayCol); d != "" {
			currentDay = d
		}
		if currentDay == "" {
			continue
		}
		timeVal := cell(row, timeCol)
		parity := cell(row, parityCol)
		if timeVal == "" || parity == "" {
			continue
		}

		for col, g := range groups {
			subject := cell(row, col+offSubject)
			if subject == "" {
				continue
			}
			g.Lessons = append(g.Lessons, Lesson{
				Day:        currentDay,
				Time:       timeVal,
				Parity:     parity,
				Subject:    subject,
				Building:   cell(row, col+offBuilding),
				Room1:      cell(row, col+offRoom1),
				Room2:      cell(row, col+offRoom2),
				Kind:       cell(row, col+offKind),
				Department: cell(row, col+offDepartment),
				Position:   cell(row, col+offPosition),
				Teacher:    cell(row, col+offTeacher),
			})
		}
	}

	result := make([]Group, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}
	return result
}

// courseFromSheetName extracts the leading course number from a sheet name like "2 курс".
func courseFromSheetName(name string) int {
	i := 0
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0
	}
	n, err := strconv.Atoi(name[:i])
	if err != nil {
		return 0
	}
	return n
}
