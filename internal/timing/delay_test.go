package timing

import (
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
