package Schedule

import (
	"sort"
	"strings"
)

// minSubstringLength is the shortest fragment the partial-match index stores.
// One character would match almost every group and is never a useful query.
const minSubstringLength = 2

// index is an immutable set of hash tables built once per refresh, so every
// lookup at request time is an O(1) map hit instead of a scan over all groups.
//
// Three tables, from most to least specific:
//
//	byKey    - canonical key (numeric code, or name when there is no code)
//	byAlias  - any exact spelling of a group: its code and its name
//	byPart   - every substring of code and name, for partial queries
//
// Values are indices into groups, not copies, so the tables stay small.
type index struct {
	groups  []Group
	byKey   map[string]int
	byAlias map[string][]int
	byPart  map[string][]int
}

func newIndex(groups []Group) *index {
	// A stable order makes lookup results deterministic across refreshes.
	sort.Slice(groups, func(i, j int) bool { return groups[i].Key() < groups[j].Key() })

	idx := &index{
		groups:  groups,
		byKey:   make(map[string]int, len(groups)),
		byAlias: make(map[string][]int, len(groups)*2),
		byPart:  make(map[string][]int, len(groups)*32),
	}

	for i, g := range groups {
		idx.byKey[normalize(g.Key())] = i

		for _, alias := range []string{g.Code, g.Name} {
			if alias == "" {
				continue
			}
			a := normalize(alias)
			idx.byAlias[a] = appendUnique(idx.byAlias[a], i)
			for _, part := range substrings(a) {
				idx.byPart[part] = appendUnique(idx.byPart[part], i)
			}
		}
	}
	return idx
}

// substrings returns every distinct fragment of s at least minSubstringLength
// long. Group codes and names are ~7-10 characters, so this is a few dozen
// short keys per group - cheap to build and tiny to hold.
func substrings(s string) []string {
	r := []rune(s)
	out := make([]string, 0, len(r)*len(r)/2)
	seen := make(map[string]struct{}, cap(out))
	for i := 0; i < len(r); i++ {
		for j := i + minSubstringLength; j <= len(r); j++ {
			part := string(r[i:j])
			if _, dup := seen[part]; dup {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}

func appendUnique(list []int, v int) []int {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

// get resolves a canonical key.
func (idx *index) get(key string) (Group, bool) {
	i, ok := idx.byKey[normalize(key)]
	if !ok {
		return Group{}, false
	}
	return idx.groups[i], true
}

// findExact resolves a query that spells out a whole code or name.
func (idx *index) findExact(query string) (Group, bool) {
	q := normalize(query)
	if q == "" {
		return Group{}, false
	}
	if i, ok := idx.byKey[q]; ok {
		return idx.groups[i], true
	}
	// A name shared by several groups is not an exact match to any one of them.
	if hits := idx.byAlias[q]; len(hits) == 1 {
		return idx.groups[hits[0]], true
	}
	return Group{}, false
}

// findMatches returns every group whose code or name contains the query.
func (idx *index) findMatches(query string, limit int) []Group {
	q := normalize(query)
	if len(q) < minSubstringLength {
		return nil
	}

	hits := idx.byPart[q]
	if len(hits) == 0 {
		return nil
	}

	out := make([]Group, 0, len(hits))
	for _, i := range hits {
		out = append(out, idx.groups[i])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// size reports how many groups the index holds.
func (idx *index) size() int { return len(idx.groups) }

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
