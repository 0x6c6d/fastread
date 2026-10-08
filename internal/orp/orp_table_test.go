package orp

import (
	"strings"
	"testing"
)

type orpCase struct {
	word  string
	pos   int
	idx   int
	focus string
}

var orpCases = []orpCase{
	{"a", 0, 0, "a"},
	{"ab", 1, 1, "b"},
	{"hello", 1, 1, "e"},
	{"wonderful", 2, 2, "n"},
	{"reading,", 2, 2, "a"},
	{"\"extraordinary\"", 3, 4, "r"},
	{"internationally", 4, 4, "r"},
	{"--", 0, 0, "-"},
	{"—", 0, 0, "—"},
	{"...", 1, 1, "."},
	{"…", 0, 0, "…"},
	{"héllo", 1, 1, "é"},
	{"héllo", 1, 1, "é"},
	{"naïve", 1, 1, "a"},
	{"«naïve»", 1, 2, "a"},
	{"¿Qué?", 1, 2, "u"},
	{"don't", 1, 1, "o"},
	{"\U0001F44D", 0, 0, "\U0001F44D"},
	{"\U0001F44D\U0001F44D\U0001F44D", 1, 1, "\U0001F44D"},
	{"\U0001F469‍\U0001F469‍\U0001F467", 0, 0, "\U0001F469‍\U0001F469‍\U0001F467"},
	{"\U0001F44D\U0001F3FD", 0, 0, "\U0001F44D\U0001F3FD"},
	{"ok\U0001F44D", 1, 1, "k"},
	{"\U0001F1E9\U0001F1EA", 0, 0, "\U0001F1E9\U0001F1EA"},
	{"1️⃣", 0, 0, "1️⃣"},
	{"日本語の本", 1, 1, "本"},
	{"한국어", 1, 1, "국"},
	{"2026", 1, 1, "0"},
	{"1,000,000", 2, 3, "0"},
	{"abcde", 1, 1, "b"},
	{"abcdef", 2, 2, "c"},
	{"abcdefghi", 2, 2, "c"},
	{"abcdefghij", 3, 3, "d"},
	{"abcdefghijklm", 3, 3, "d"},
	{"abcdefghijklmn", 4, 4, "e"},
	{strings.Repeat("abcdefghij", 4), 4, 4, "e"},
	{"\"", 0, 0, "\""},
	{"", 0, 0, ""},
}

func TestPosition(t *testing.T) {
	for _, c := range orpCases {
		if got := Position(c.word); got != c.pos {
			t.Errorf("Position(%q) = %d, want %d", c.word, got, c.pos)
		}
	}
}

func TestIndex(t *testing.T) {
	for _, c := range orpCases {
		cs := Clusters(c.word)
		if got := Index(c.word); got != c.idx {
			t.Errorf("Index(%q) = %d, want %d", c.word, got, c.idx)
			continue
		}
		if c.word != "" {
			if c.idx >= len(cs) || cs[c.idx] != c.focus {
				t.Errorf("Clusters(%q)[%d] != %q (clusters %q)", c.word, c.idx, c.focus, cs)
			}
		}
		if j := strings.Join(cs, ""); j != c.word {
			t.Errorf("Join(Clusters(%q)) = %q", c.word, j)
		}
	}
}

func TestIndexInRange(t *testing.T) {
	words := []string{"\xff\xfe", "a\xffb", "́", "x‍", strings.Repeat("éx", 100)}
	for _, c := range orpCases {
		words = append(words, c.word)
	}
	if len(words) < 30 {
		t.Fatalf("only %d words", len(words))
	}
	for _, w := range words {
		idx := Index(w)
		limit := len(Clusters(w))
		if limit < 1 {
			limit = 1
		}
		if idx < 0 || idx >= limit {
			t.Errorf("Index(%q) = %d out of [0,%d)", w, idx, limit)
		}
		if p := Position(w); p < 0 || p > 4 {
			t.Errorf("Position(%q) = %d", w, p)
		}
	}
}
