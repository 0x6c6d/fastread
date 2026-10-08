package orp

import "testing"

func TestPositionBasic(t *testing.T) {
	tests := []struct {
		word string
		want int
	}{
		{"a", 0},
		{"ab", 1},
		{"hello", 1},
		{"wonderful", 2},
		{"reading,", 2},
		{`"extraordinary"`, 3},
		{"internationally", 4},
		{"--", 0},
		{"—", 0},
		{"...", 1},
		{"", 0},
	}
	for _, tt := range tests {
		if got := Position(tt.word); got != tt.want {
			t.Errorf("Position(%q) = %d, want %d", tt.word, got, tt.want)
		}
	}
}

func TestIndexBasic(t *testing.T) {
	tests := []struct {
		word string
		want int
	}{
		{"a", 0},
		{"hello", 1},
		{`"hello"`, 2},
		{"(wonderful)", 3},
		{`"extraordinary"`, 4},
		{"--", 0},
		{"...", 1},
		{"don't", 1},
		{"", 0},
	}
	for _, tt := range tests {
		if got := Index(tt.word); got != tt.want {
			t.Errorf("Index(%q) = %d, want %d", tt.word, got, tt.want)
		}
	}
}

func TestNoPanic(t *testing.T) {
	words := []string{
		"\xff\xfe",
		"\u0301",
		"\U0001F469\u200D\U0001F469\u200D\U0001F467",
		"e\u0301e\u0301",
		"\u00e9\u00e9",
	}
	for _, w := range words {
		_ = Position(w)
		cs := Clusters(w)
		idx := Index(w)
		limit := len(cs)
		if limit < 1 {
			limit = 1
		}
		if idx < 0 || idx >= limit {
			t.Errorf("Index(%q) = %d, want 0 <= idx < %d", w, idx, limit)
		}
		joined := ""
		for _, c := range cs {
			joined += c
		}
		if joined != w {
			t.Errorf("Clusters(%q) = %q does not reassemble the word", w, cs)
		}
	}
}
