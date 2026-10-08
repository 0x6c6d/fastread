//go:build ignore

// Spec value checks for the final Definition of Done (supervisor-owned; workers never edit it).
// The leading underscore keeps it out of every Go package. scripts/verify/T069.sh copies it
// into a temporary dot directory inside the repository (so the internal packages are
// importable while ./... never sees it) and runs it with `go run <dir>/main.go`.
//
// It checks README §8 AC7 (orp.Position/orp.Index) and AC8 (timing.Delay/DelayPara) values
// directly against the built packages, independent of the unit tests, prints one line per
// result and exits 1 if any expectation failed.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0x6c6d/fastread/internal/orp"
	"github.com/0x6c6d/fastread/internal/timing"
	"github.com/rivo/uniseg"
)

func main() {
	bad := 0
	check := func(ok bool, format string, a ...any) {
		if !ok {
			bad++
			fmt.Printf("FAIL "+format+"\n", a...)
		}
	}
	ms := time.Millisecond

	// AC8 / R10 at 300 wpm (base 200 ms).
	delays := []struct {
		word string
		para bool
		want time.Duration
	}{
		{"word", false, 200 * ms},
		{"word.", false, 400 * ms},
		{"word!", false, 400 * ms},
		{"word?", false, 400 * ms},
		{"word,", false, 300 * ms},
		{"word;", false, 300 * ms},
		{"word:", false, 300 * ms},
		{"abcdefgh", false, 200 * ms},
		{"abcdefghijkl", false, 240 * ms},
		{strings.Repeat("a", 30), false, 300 * ms},
		{"word", true, 500 * ms},
		{"word.", true, 500 * ms},
		{"word,", true, 500 * ms},
	}
	for _, d := range delays {
		got := timing.DelayPara(d.word, 300, d.para)
		check(got == d.want, "DelayPara(%q, 300, %v) = %v, want %v", d.word, d.para, got, d.want)
		if !d.para {
			got = timing.Delay(d.word, 300)
			check(got == d.want, "Delay(%q, 300) = %v, want %v", d.word, got, d.want)
		}
	}
	for _, c := range []struct {
		wpm  int
		want time.Duration
	}{{50, 1200 * ms}, {60, 1000 * ms}, {600, 100 * ms}, {1500, 40 * ms}} {
		got := timing.Delay("word", c.wpm)
		check(got == c.want, "Delay(\"word\", %d) = %v, want %v", c.wpm, got, c.want)
	}

	// AC7 / R9: Position = focus rank among letter/digit clusters.
	positions := []struct {
		word string
		want int
	}{
		{"a", 0}, {"ab", 1}, {"hello", 1}, {"wonderful", 2}, {"reading,", 2},
		{"\"extraordinary\"", 3}, {"internationally", 4}, {"héllo", 1}, {"naïve", 1},
		{"naïve", 1}, {"abcdef", 2}, {"abcdefghi", 2}, {"abcdefghij", 3},
		{"abcdefghijklm", 3}, {"abcdefghijklmn", 4}, {strings.Repeat("x", 40), 4},
	}
	for _, p := range positions {
		got := orp.Position(p.word)
		check(got == p.want, "Position(%q) = %d, want %d", p.word, got, p.want)
	}
	// Index = absolute grapheme-cluster index; zero letters/digits → middle cluster (n-1)/2.
	indexes := []struct {
		word string
		want int
	}{
		{"--", 0}, {"—", 0}, {"---", 1}, {"-----", 2}, {"\"extraordinary\"", 4},
		{"reading,", 2}, {"(a)", 1}, {"hello", 1},
	}
	for _, x := range indexes {
		got := orp.Index(x.word)
		check(got == x.want, "Index(%q) = %d, want %d", x.word, got, x.want)
	}
	// Never crash, always a valid cluster index (emoji, wide, combining, flags).
	for _, w := range []string{"👍", "👍🏽ok", "日本語", "é́", "🇩🇪x", "a‍b", "١٢٣"} {
		n := uniseg.GraphemeClusterCount(w)
		func() {
			defer func() {
				if r := recover(); r != nil {
					check(false, "Index(%q) panicked: %v", w, r)
				}
			}()
			i := orp.Index(w)
			check(i >= 0 && i < n, "Index(%q) = %d, outside [0,%d)", w, i, n)
			_ = orp.Position(w)
		}()
	}

	if bad > 0 {
		fmt.Printf("dodcheck: %d expectation(s) failed\n", bad)
		os.Exit(1)
	}
	fmt.Printf("dodcheck: all AC7/AC8 expectations hold (%d delay rows, %d position rows, %d index rows)\n",
		len(delays)+4, len(positions), len(indexes))
}
