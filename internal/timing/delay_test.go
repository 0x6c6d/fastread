package timing

import (
	"strings"
	"testing"
	"time"
)

func TestDelayBase(t *testing.T) {
	tests := []struct {
		name string
		word string
		wpm  int
		want time.Duration
	}{
		{"300 wpm", "hello", 300, 200 * time.Millisecond},
		{"50 wpm", "world", 50, 1200 * time.Millisecond},
		{"1500 wpm", "reading", 1500, 40 * time.Millisecond},
		{"600 wpm", "fast", 600, 100 * time.Millisecond},
		{"zero clamped to min", "word", 0, 1200 * time.Millisecond},
		{"negative clamped to min", "text", -5, 1200 * time.Millisecond},
		{"huge clamped to max", "speed", 99999, 40 * time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Delay(tc.word, tc.wpm); got != tc.want {
				t.Errorf("Delay(%q, %d) = %v, want %v", tc.word, tc.wpm, got, tc.want)
			}
		})
	}

	for _, w := range []string{"hello", "a", "reading"} {
		if got, want := DelayPara(w, 300, false), Delay(w, 300); got != want {
			t.Errorf("DelayPara(%q, 300, false) = %v, want Delay = %v", w, got, want)
		}
	}
}

type delayRow struct {
	word string
	wpm  int
	ms   int
}

var delayRows = []delayRow{
	{"word", 300, 200}, {"word.", 300, 400}, {"word,", 300, 300}, {"word;", 300, 300},
	{"word:", 300, 300}, {"word!", 300, 400}, {"word?", 300, 400}, {"word…", 300, 400},
	{"etc...", 300, 400}, {"?!", 300, 400}, {"\"word.\"", 300, 400}, {"(word),", 300, 300},
	{"word?)", 300, 400}, {"word»", 300, 200}, {"’word’", 300, 200}, {"don't", 300, 200},
	{"—", 300, 200}, {"--", 300, 200}, {"...", 300, 400}, {"naïve.", 300, 400},
	{"héllo", 300, 200}, {"2026.", 300, 400}, {"abcdefgh", 300, 200}, {"wonderful", 300, 210},
	{"abcdefghijkl", 300, 240}, {strings.Repeat("a", 18), 300, 300},
	{strings.Repeat("a", 30), 300, 300}, {"extraordinary.", 300, 500},
	{"abcdefghijkl,", 300, 360}, {"wonderful", 600, 105}, {"word", 1000, 60},
	{"word.", 1000, 120}, {"word.", 450, 267}, {"word,", 450, 200},
	{"abcdefghijk", 450, 153}, {"word.", 0, 2400}, {"word.", 99999, 80},
}

func TestDelay(t *testing.T) {
	for _, r := range delayRows {
		want := time.Duration(r.ms) * time.Millisecond
		if got := Delay(r.word, r.wpm); got != want {
			t.Errorf("Delay(%q, %d) = %v, want %v", r.word, r.wpm, got, want)
		}
		if got := DelayPara(r.word, r.wpm, false); got != want {
			t.Errorf("DelayPara(%q, %d, false) = %v, want %v", r.word, r.wpm, got, want)
		}
		if want <= 0 {
			t.Errorf("non-positive want for %q", r.word)
		}
		if p := DelayPara(r.word, r.wpm, true); p < want {
			t.Errorf("DelayPara(%q, %d, true) = %v < %v", r.word, r.wpm, p, want)
		}
	}
}

func TestDelayPara(t *testing.T) {
	tests := []delayRow{
		{"word", 300, 500}, {"word.", 300, 500}, {"word,", 300, 500}, {"word!", 300, 500},
		{"wonderful", 300, 525}, {strings.Repeat("a", 30), 300, 750},
		{"extraordinary.", 300, 625}, {"—", 300, 500}, {"word.", 450, 333},
	}
	for _, r := range tests {
		want := time.Duration(r.ms) * time.Millisecond
		if got := DelayPara(r.word, r.wpm, true); got != want {
			t.Errorf("DelayPara(%q, %d, true) = %v, want %v", r.word, r.wpm, got, want)
		}
	}
}
