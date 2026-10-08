package tui

import (
	"strings"
	"testing"
	"time"
)

func TestRenderFast(t *testing.T) {
	bound := 20 * time.Millisecond
	if raceEnabled {
		bound *= 10
	}
	words := []string{"Hello", "wonderful", "naïve", "日本語の本", strings.Repeat("abcdefghij", 6)}
	frames := [][2]int{{80, 24}, {200, 60}}
	for size := 1; size <= 5; size++ {
		for _, word := range words {
			for _, fr := range frames {
				m := Model{Word: word, Size: size, ShowProgress: true, ShowHelp: true,
					Index: 41, Total: 1000, WPM: 300, EffectiveWPM: 300}
				start := time.Now()
				for i := 0; i < 50; i++ {
					f := Render(m, fr[0], fr[1])
					_ = Encode(f, ColorTrue)
				}
				avg := time.Since(start) / 50
				if avg >= bound {
					t.Errorf("size %d word %q %dx%d: avg %v >= %v", size, word, fr[0], fr[1], avg, bound)
				}
			}
		}
	}
	m := Model{Word: strings.Repeat("a", 100000), Size: 1, Part: 1 << 20, ShowProgress: true, ShowHelp: true,
		Index: 1, Total: 10, WPM: 300, EffectiveWPM: 300}
	start := time.Now()
	f := Render(m, 80, 24)
	_ = Encode(f, ColorTrue)
	if d := time.Since(start); d >= 10*bound {
		t.Errorf("long word: %v >= %v", d, 10*bound)
	}
}

func BenchmarkRenderFrame(b *testing.B) {
	m := Model{Word: "wonderful", Size: 3, ShowProgress: true, ShowHelp: true, Index: 41, Total: 1000, EffectiveWPM: 300}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f := Render(m, 120, 40)
		_ = Encode(f, ColorTrue)
	}
}
