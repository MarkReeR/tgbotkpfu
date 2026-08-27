package Schedule

// Lesson is a single class occurrence for one group, one weekday, one time slot and one week parity.
type Lesson struct {
	Day        string // "Понедельник".."Суббота"
	Time       string // "8:00"
	Parity     string // "в" or "н"
	Subject    string
	Building   string
	Room1      string
	Room2      string
	Kind       string // Вид (лекция/пр./лаб.)
	Department string
	Position   string
	Teacher    string
}

// Group is a single study group parsed from one of the course sheets.
// At least one of Code and Name is always set: most sheets carry the numeric
// code, but some blocks are labelled only with the friendly name.
type Group struct {
	Code    string // numeric group code, e.g. "8251160"
	Name    string // human-friendly name, e.g. "18.1-542"
	Course  int
	Lessons []Lesson
}

// Key is the stable identifier used to store and look up a group. It is the
// numeric code when there is one, and the friendly name otherwise - so it stays
// the same across refreshes and is safe to persist in the database.
func (g Group) Key() string {
	if g.Code != "" {
		return g.Code
	}
	return g.Name
}

// DisplayName returns the code together with the friendly name when both are known.
func (g Group) DisplayName() string {
	switch {
	case g.Code == "":
		return g.Name
	case g.Name == "":
		return g.Code
	default:
		return g.Code + " (" + g.Name + ")"
	}
}
