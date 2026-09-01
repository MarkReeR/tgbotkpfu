package Bot

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestClampMessage(t *testing.T) {
	short := "коротко"
	if got := clampMessage(short); got != short {
		t.Errorf("короткое сообщение изменено: %q", got)
	}

	// Exactly at the limit must pass through untouched.
	exact := strings.Repeat("я", telegramMessageLimit)
	if got := clampMessage(exact); got != exact {
		t.Errorf("сообщение ровно в лимит обрезано: было %d, стало %d",
			utf8.RuneCountInString(exact), utf8.RuneCountInString(got))
	}

	// One over the limit gets trimmed, and the result must fit.
	long := strings.Repeat("строка расписания\n", 500)
	got := clampMessage(long)
	if n := utf8.RuneCountInString(got); n > telegramMessageLimit {
		t.Errorf("после обрезки %d символов, лимит %d", n, telegramMessageLimit)
	}
	if !strings.HasSuffix(got, "сообщение обрезано") {
		t.Errorf("нет пометки об обрезке: ...%q", got[len(got)-40:])
	}

	// Cyrillic is two bytes per rune: the limit is counted in characters, so a
	// message of 3000 Cyrillic letters must survive despite being 6000 bytes.
	cyrillic := strings.Repeat("щ", 3000)
	if got := clampMessage(cyrillic); got != cyrillic {
		t.Errorf("кириллица обрезана по байтам, а не по символам: %d -> %d",
			utf8.RuneCountInString(cyrillic), utf8.RuneCountInString(got))
	}
}
