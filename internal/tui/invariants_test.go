package tui

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/0x6c6d/fastread/internal/tui/glyph"
)

var invEscRe = regexp.MustCompile(`\x1b\[[0-9;]*[Hm]`)

func invBadRune(r rune) bool {
	return r <= 0x1f || (r >= 0x7f && r <= 0x9f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

func TestRenderInvariants(t *testing.T) {
	pool := []string{"a", "b", "c", "x", "y", "z", "H", "é", "ï", "0", "7", ",", ".", "-", "\"", "(", ")",
		"«", "»", "¿", "—", "“", "”", "日", "本", "한", "\U0001F44D",
		"\U0001F469‍\U0001F469‍\U0001F467", "\U0001F1E9\U0001F1EA",
		"é", "́", "​", "\x1b", "‮", "\x9b", "\t"}
	rng := rand.New(rand.NewPCG(1, 2))
	const n = 3000
	for i := 0; i < n; i++ {
		var sb strings.Builder
		for k := 1 + rng.IntN(40); k > 0; k-- {
			sb.WriteString(pool[rng.IntN(len(pool))])
		}
		word := sb.String()
		m := Model{
			Word: word, Size: rng.IntN(8) - 1, Part: rng.IntN(7) - 1,
			Paused: rng.IntN(2) == 0, ShowProgress: rng.IntN(2) == 0, ShowHelp: rng.IntN(2) == 0,
			Index: rng.IntN(1000), Total: 1 + rng.IntN(1000), WPM: rng.IntN(1501), EffectiveWPM: rng.IntN(1501),
		}
		w, h := rng.IntN(203)-2, rng.IntN(63)-2
		f := Render(m, w, h)
		if len(f.Cells) != max(h, 0) {
			t.Fatalf("case %d %q %+v %dx%d: rows %d", i, word, m, w, h, len(f.Cells))
		}
		for y, row := range f.Cells {
			if len(row) != max(w, 0) {
				t.Fatalf("case %d %q %+v %dx%d: row %d len %d", i, word, m, w, h, y, len(row))
			}
		}
		minX := -1
		ticks := 0
		for y, row := range f.Cells {
			for x, c := range row {
				if c.Style == StyleFocus && (minX < 0 || x < minX) {
					minX = x
				}
				if c.Style == StyleTick {
					ticks++
					if x != w/2 || y < 1 || y > h-2 {
						t.Fatalf("case %d %q %+v %dx%d: tick at %d,%d", i, word, m, w, h, x, y)
					}
				}
				if c.Cont && (x == 0 || row[x-1].Cont) {
					t.Fatalf("case %d %q %+v %dx%d: bad Cont at %d,%d", i, word, m, w, h, x, y)
				}
			}
		}
		if w < 20 || h < 5 {
			if f.Size != 0 || f.Parts != 0 || minX >= 0 || ticks != 0 {
				t.Fatalf("case %d %q %+v %dx%d: small frame not empty: size %d parts %d", i, word, m, w, h, f.Size, f.Parts)
			}
		} else {
			if want := EffectiveSize(word, m.Size, w, h); f.Size != want {
				t.Fatalf("case %d %q %+v %dx%d: size %d want %d", i, word, m, w, h, f.Size, want)
			}
			if f.Size < 1 || f.Size > max(1, min(m.Size, 5)) {
				t.Fatalf("case %d %q %+v %dx%d: size %d out of range", i, word, m, w, h, f.Size)
			}
			if want := len(Split(word, f.Size, w)); f.Parts != want {
				t.Fatalf("case %d %q %+v %dx%d: parts %d want %d", i, word, m, w, h, f.Parts, want)
			}
			if minX < 0 {
				t.Fatalf("case %d %q %+v %dx%d: no focus cell", i, word, m, w, h)
			}
			col := minX
			if f.Size >= 2 {
				col += glyph.Width(f.Size) / 2
			}
			if col != w/2 {
				t.Fatalf("case %d %q %+v %dx%d: focus col %d want %d", i, word, m, w, h, col, w/2)
			}
			if ticks != 2 {
				t.Fatalf("case %d %q %+v %dx%d: ticks %d", i, word, m, w, h, ticks)
			}
		}
		for _, mode := range []ColorMode{ColorNone, Color16, Color256, ColorTrue} {
			out := Encode(f, mode)
			if !utf8.Valid(out) {
				t.Fatalf("case %d %q %+v %dx%d mode %d: invalid UTF-8", i, word, m, w, h, mode)
			}
			for _, r := range invEscRe.ReplaceAllString(string(out), "") {
				if invBadRune(r) {
					t.Fatalf("case %d %q %+v %dx%d mode %d: bad rune %U", i, word, m, w, h, mode, r)
				}
			}
		}
	}
	t.Logf("cases: %d", n)
}
