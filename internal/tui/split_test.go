package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/orp"
	"github.com/0x6c6d/fastread/internal/tui/glyph"
	"github.com/rivo/uniseg"
)

// stepFits is an independent implementation of the fit rule.
func stepFits(t string, size, w int) bool {
	cs := orp.Clusters(t)
	fi := orp.Index(t)
	if size >= 2 {
		W := glyph.Width(size)
		return fi*W <= w/2-W/2 && w/2-W/2+(len(cs)-fi)*W <= w
	}
	l, r := 0, 0
	for k, c := range cs {
		cw := uniseg.StringWidth(c)
		if cw < 1 {
			cw = 1
		}
		if k < fi {
			l += cw
		} else {
			r += cw
		}
	}
	return l <= w/2 && w/2+r <= w
}

func rowText(f Frame) string {
	var sb strings.Builder
	for _, c := range f.Cells[f.H/2] {
		if c.Cont {
			continue
		}
		if c.Text == "" {
			sb.WriteByte(' ')
		} else {
			sb.WriteString(c.Text)
		}
	}
	return strings.TrimSpace(sb.String())
}

func checkSteps(t *testing.T, word string, size, w, h int, steps []string) {
	t.Helper()
	var sb strings.Builder
	cs := orp.Clusters(word)
	for i, s := range steps {
		if !stepFits(s, size, w) {
			t.Fatalf("step %d %q does not fit (size %d w %d)", i, s, size, w)
		}
		if i < len(steps)-1 {
			if !strings.HasSuffix(s, "-") {
				t.Fatalf("step %d %q lacks hyphen", i, s)
			}
			s = strings.TrimSuffix(s, "-")
			// maximal: one more cluster plus hyphen (or the rest) must not fit
			sb.WriteString(s)
			n := len(orp.Clusters(sb.String()))
			if n < len(cs) {
				start := n - len(orp.Clusters(s))
				next := strings.Join(cs[start:n+1], "")
				if n+1 < len(cs) {
					next += "-"
				}
				if stepFits(next, size, w) {
					t.Fatalf("step %d %q is not maximal", i, steps[i])
				}
			}
			continue
		}
		sb.WriteString(s)
	}
	if sb.String() != word {
		t.Fatalf("reassembled %q != %q", sb.String(), word)
	}
}

func TestSplitLongWord(t *testing.T) {
	w60 := strings.Repeat("abcdefghij", 6)
	got := Split(w60, 1, 20)
	want := []string{"abcdefghijab-", "cdefghijabcd-", "efghijabcdef-", "ghijabcdefgh-", "ijabcdefghij"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("size 1: got %q", got)
	}
	g2 := Split(w60, 2, 20)
	if len(g2) != 20 || g2[0] != "abc-" || g2[1] != "def-" || g2[2] != "ghi-" || g2[3] != "jab-" || g2[19] != "hij" {
		t.Fatalf("size 2: %d %q", len(g2), g2)
	}
	for _, s := range []int{3, 4, 5} {
		e := EffectiveSize(w60, s, 20, 24)
		if e != 3 {
			t.Fatalf("size %d effective %d", s, e)
		}
		g := Split(w60, e, 20)
		if len(g) != 30 || g[0] != "ab-" || g[1] != "cd-" || g[29] != "ij" {
			t.Fatalf("size %d: %d %q", s, len(g), g)
		}
	}
	words := []string{w60, "state-of-the-art-technology-and-more",
		strings.Repeat("日本語", 12), "Donaudampfschifffahrtsgesellschaftskapitän",
		strings.Repeat("👍", 12), strings.Repeat("é", 30), "Hello"}
	for _, word := range words {
		for _, w := range []int{20, 80} {
			for size := 1; size <= 5; size++ {
				e := EffectiveSize(word, size, w, 24)
				steps := Split(word, e, w)
				f := Render(Model{Word: word, Size: size}, w, 24)
				if f.Parts != len(steps) {
					t.Fatalf("%q size %d w %d: Parts %d != %d", word, size, w, f.Parts, len(steps))
				}
				checkSteps(t, word, e, w, 24, steps)
				if e == 1 {
					for p, s := range steps {
						if got := rowText(Render(Model{Word: word, Size: size, Part: p}, w, 24)); got != s {
							t.Fatalf("%q part %d reads %q want %q", word, p, got, s)
						}
					}
				}
				first := Render(Model{Word: word, Size: size, Part: -1}, w, 24)
				last := Render(Model{Word: word, Size: size, Part: 99}, w, 24)
				if rowText(first) != rowText(Render(Model{Word: word, Size: size}, w, 24)) {
					t.Fatalf("Part -1 is not step 0")
				}
				if e == 1 && rowText(last) != steps[len(steps)-1] {
					t.Fatalf("Part 99 is not the last step")
				}
			}
		}
	}
	if n := len(Split("Hello", 3, 80)); n != 1 {
		t.Fatalf("Hello at 80: %d", n)
	}
	if n := len(Split("Hello", 3, 20)); n != 2 {
		t.Fatalf("Hello at 20 size 3: %d", n)
	}
	if got := Split("", 1, 20); len(got) != 1 || got[0] != "" {
		t.Fatalf("empty: %q", got)
	}
}

func TestSplitLinear(t *testing.T) {
	limit := time.Second
	if raceEnabled {
		limit = 10 * time.Second
	}
	word := strings.Repeat("a", 100000)
	for _, size := range []int{1, 3} {
		start := time.Now()
		steps := Split(word, size, 80)
		if d := time.Since(start); d > limit {
			t.Fatalf("size %d took %v", size, d)
		}
		if strings.Join(strings.Split(strings.Join(steps, "|"), "-|"), "") != word {
			t.Fatalf("size %d does not reassemble", size)
		}
	}
}

func TestFocusColumn(t *testing.T) {
	w60 := strings.Repeat("abcdefghij", 6)
	words := []string{"a", "I", "ab", "Hello", "wonderful", "reading,", "\"extraordinary\"",
		"internationally", "(wonderful)", "--", "—", "...", "naïve", "naïve", "¿Qué?",
		"don't", "1,000,000", "2026", "日本語の本", "한국어", "👍", "👩‍👩‍👧", "🇩🇪",
		"ok👍", "été", "́abc", "Ωmega", "“quoted”", "ÆØÅ", w60,
		"Donaudampfschifffahrtsgesellschaftskapitän", "state-of-the-art-technology-and-more"}
	frames := 0
	for _, word := range words {
		for size := 1; size <= 5; size++ {
			for _, w := range []int{80, 121} {
				f0 := Render(Model{Word: word, Size: size}, w, 24)
				for p := 0; p < f0.Parts; p++ {
					f := Render(Model{Word: word, Size: size, Part: p}, w, 24)
					frames++
					minX, count := -1, 0
					for _, row := range f.Cells {
						for x, c := range row {
							if c.Style == StyleFocus {
								count++
								if minX < 0 || x < minX {
									minX = x
								}
							}
						}
					}
					col := minX
					if f.Size >= 2 {
						col += glyph.Width(f.Size) / 2
						if count != glyph.Width(f.Size)*glyph.Rows(f.Size) {
							t.Fatalf("%q size %d w %d part %d: %d focus cells", word, size, w, p, count)
						}
					} else if count != 1 {
						t.Fatalf("%q size %d w %d part %d: %d focus cells", word, size, w, p, count)
					}
					if col != w/2 {
						t.Fatalf("%q size %d w %d part %d: focus column %d want %d", word, size, w, p, col, w/2)
					}
				}
			}
		}
	}
	t.Logf("frames checked: %d", frames)
}
