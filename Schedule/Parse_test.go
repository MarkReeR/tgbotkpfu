package Schedule

import "testing"

func TestParseGroupHeader(t *testing.T) {
	cases := []struct {
		in         string
		code, name string
		ok         bool
	}{
		{"8261160 (18.1-642)", "8261160", "18.1-642", true},
		{"8241160", "8241160", "", true},
		// Name-only header: the digits are not the whole token, so it must not
		// be read as the code "18" - that used to collapse five groups into one.
		{"18.1-512", "", "18.1-512", true},
		{"  3221103  ", "3221103", "", true},
		{"", "", "", false},
		{"Дисциплина", "", "", false},
	}

	for _, c := range cases {
		code, name, ok := parseGroupHeader(c.in)
		if code != c.code || name != c.name || ok != c.ok {
			t.Errorf("parseGroupHeader(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, code, name, ok, c.code, c.name, c.ok)
		}
	}
}

func TestGroupKeyAndDisplayName(t *testing.T) {
	cases := []struct {
		g       Group
		key     string
		display string
	}{
		{Group{Code: "8261160", Name: "18.1-642"}, "8261160", "8261160 (18.1-642)"},
		{Group{Code: "8241160"}, "8241160", "8241160"},
		{Group{Name: "18.1-512"}, "18.1-512", "18.1-512"},
	}

	for _, c := range cases {
		if got := c.g.Key(); got != c.key {
			t.Errorf("Key() = %q, want %q", got, c.key)
		}
		if got := c.g.DisplayName(); got != c.display {
			t.Errorf("DisplayName() = %q, want %q", got, c.display)
		}
	}
}

func TestIndexLookups(t *testing.T) {
	idx := newIndex([]Group{
		{Code: "8261160", Name: "18.1-642"},
		{Code: "1261102", Name: "18.1-638"},
		{Name: "18.1-512"},
	})

	if g, ok := idx.get("8261160"); !ok || g.Name != "18.1-642" {
		t.Error("get by code failed")
	}
	if g, ok := idx.get("18.1-512"); !ok || g.Code != "" {
		t.Error("get by name-only key failed")
	}
	if g, ok := idx.findExact("18.1-642"); !ok || g.Code != "8261160" {
		t.Error("findExact by friendly name failed")
	}
	if _, ok := idx.findExact("нет такой"); ok {
		t.Error("findExact matched a missing group")
	}

	if got := idx.findMatches("642", 10); len(got) != 1 || got[0].Code != "8261160" {
		t.Errorf("findMatches(642) = %v", got)
	}
	// "18.1-" is shared by all three groups.
	if got := idx.findMatches("18.1-", 10); len(got) != 3 {
		t.Errorf("findMatches(18.1-) returned %d groups, want 3", len(got))
	}
	// Queries shorter than minSubstringLength are refused rather than matching everything.
	if got := idx.findMatches("1", 10); got != nil {
		t.Errorf("single-char query returned %d groups, want none", len(got))
	}
	if got := idx.findMatches("642", 0); len(got) != 1 {
		t.Errorf("limit 0 should mean unlimited, got %d", len(got))
	}
}
