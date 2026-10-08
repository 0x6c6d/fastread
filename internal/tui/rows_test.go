package tui

import (
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/tui/glyph"
)

func rowStr(f Frame, y int) string {
	var b strings.Builder
	for _, c := range f.Cells[y] {
		if c.Text == "" {
			b.WriteString(" ")
		} else {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

func TestProgressHelpFocus(t *testing.T) {
	lit := []struct {
		name string
		m    Model
		w, h int
		row  int
		want string
	}{
		{"bar287", Model{Word: "Hello", Size: 1, ShowProgress: true, Index: 4, Total: 120, EffectiveWPM: 287}, 80, 24, 23,
			strings.Repeat("━", 2) + strings.Repeat("─", 58) + " " + "word 5/120  287 wpm"},
		{"bardash", Model{Word: "Hello", Size: 1, ShowProgress: true, Index: 0, Total: 3}, 80, 24, 23,
			strings.Repeat("━", 21) + strings.Repeat("─", 43) + " " + "word 1/3  — wpm"},
		{"nobar", Model{Word: "Hello", Size: 1, ShowProgress: true, Index: 4, Total: 120, EffectiveWPM: 287}, 20, 5, 4,
			"word 5/120  287 wpm "},
		{"help40", Model{Word: "Hello", Size: 1, ShowHelp: true}, 40, 12, 0, string([]rune(HelpText)[:40])},
	}
	for _, l := range lit {
		if got := rowStr(Render(l.m, l.w, l.h), l.row); got != l.want {
			t.Errorf("%s: row %d = %q, want %q", l.name, l.row, got, l.want)
		}
	}
	words := []string{"a", "Hello", "wonderful", "naïve", "日本"}
	frames := [][2]int{{80, 24}, {40, 12}, {20, 5}}
	for _, word := range words {
		for size := 1; size <= 5; size++ {
			for _, fr := range frames {
				w, h := fr[0], fr[1]
				var base Frame
				for i := 0; i < 4; i++ {
					m := Model{Word: word, Size: size, Index: 4, Total: 120, EffectiveWPM: 287,
						ShowProgress: i&1 != 0, ShowHelp: i&2 != 0}
					f := Render(m, w, h)
					if i == 0 {
						base = f
						if f.Size >= 1 && (rowStr(f, 0) != strings.Repeat(" ", w) || rowStr(f, h-1) != strings.Repeat(" ", w)) {
							t.Errorf("%q s%d %dx%d: rows not blank", word, size, w, h)
						}
					}
					if f.Size != base.Size {
						t.Fatalf("%q s%d %dx%d: size changed", word, size, w, h)
					}
					if f.Size < 1 {
						continue
					}
					for y := 1; y <= h-2; y++ {
						for x := 0; x < w; x++ {
							if f.Cells[y][x] != base.Cells[y][x] {
								t.Fatalf("%q s%d %dx%d toggles %d: cell %d,%d changed", word, size, w, h, i, x, y)
							}
						}
					}
					minX := -1
					for y := range f.Cells {
						for x, c := range f.Cells[y] {
							if c.Style == StyleFocus && (minX < 0 || x < minX) {
								minX = x
							}
						}
					}
					if minX >= 0 {
						if f.Size >= 2 {
							minX += glyph.Width(f.Size) / 2
						}
						if minX != w/2 {
							t.Errorf("%q s%d %dx%d: focus col %d, want %d", word, size, w, h, minX, w/2)
						}
					}
					wantHelp := strings.Repeat(" ", w)
					if m.ShowHelp {
						wantHelp = string([]rune(HelpText + strings.Repeat(" ", w))[:w])
					}
					if got := rowStr(f, 0); got != wantHelp {
						t.Errorf("%q s%d %dx%d: help row %q", word, size, w, h, got)
					}
					wantProg := strings.Repeat(" ", w)
					if m.ShowProgress {
						switch w {
						case 80:
							wantProg = strings.Repeat("━", 2) + strings.Repeat("─", 58) + " word 5/120  287 wpm"
						case 20:
							wantProg = "word 5/120  287 wpm "
						default: // 40: tw=19, bw=20, filled=5*20/120=0
							wantProg = strings.Repeat("─", 20) + " word 5/120  287 wpm"
						}
					}
					if got := rowStr(f, h-1); got != wantProg {
						t.Errorf("%q s%d %dx%d: progress row %q, want %q", word, size, w, h, got, wantProg)
					}
				}
			}
		}
	}
}

func TestNoProgressFrame(t *testing.T) {
	for size := 1; size <= 5; size++ {
		m := Model{Word: "Hello", Size: size, Index: 0, Total: 3}
		f := Render(m, 80, 24)
		if rowStr(f, 23) != strings.Repeat(" ", 80) {
			t.Errorf("size %d: row 23 not blank", size)
		}
		out := string(Encode(f, Color16))
		if strings.Contains(out, "word 1/") || strings.Contains(out, "wpm") {
			t.Errorf("size %d: progress text present", size)
		}
		m.ShowProgress = true
		if out := string(Encode(Render(m, 80, 24), Color16)); !strings.Contains(out, "word 1/3") {
			t.Errorf("size %d: progress missing", size)
		}
	}
	f := Render(Model{Word: "Hello", Size: 1, ShowProgress: true, ShowHelp: true, Total: 3}, 19, 4)
	out := string(Encode(f, Color16))
	if strings.Contains(out, HelpText) || strings.Contains(out, "word ") {
		t.Errorf("too-small frame has rows: %q", out)
	}
}
