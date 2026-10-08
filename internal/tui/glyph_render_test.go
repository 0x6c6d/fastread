package tui

import (
	"testing"

	"github.com/0x6c6d/fastread/internal/orp"
	"github.com/0x6c6d/fastread/internal/tui/glyph"
)

func focusRows(f Frame) (minY, maxY, n int) {
	minY, maxY = -1, -1
	for y, row := range f.Cells {
		for _, c := range row {
			if c.Style == StyleFocus {
				if minY < 0 {
					minY = y
				}
				maxY = y
				n++
			}
		}
	}
	return
}

func TestSizeFallback(t *testing.T) {
	cases := []struct{ size, w, h, want int }{
		{5, 80, 24, 5}, {5, 29, 24, 5}, {5, 28, 24, 4}, {5, 23, 24, 4},
		{5, 22, 24, 3}, {5, 20, 24, 3}, {5, 80, 13, 5}, {5, 80, 12, 4},
		{5, 80, 11, 4}, {5, 80, 10, 3}, {5, 80, 9, 3}, {5, 40, 8, 2},
		{5, 40, 7, 2}, {5, 40, 6, 1}, {5, 40, 5, 1}, {5, 19, 4, 0},
		{5, 19, 24, 0}, {5, 80, 4, 0}, {3, 80, 24, 3}, {1, 80, 24, 1},
		{0, 80, 24, 1}, {-2, 80, 24, 1}, {9, 80, 24, 5},
	}
	for _, c := range cases {
		f := Render(Model{Word: "Hello", Size: c.size}, c.w, c.h)
		if f.Size != c.want || f.Size != EffectiveSize("Hello", c.size, c.w, c.h) {
			t.Errorf("size %d at %dx%d: got %d want %d", c.size, c.w, c.h, f.Size, c.want)
		}
		if c.want == 0 {
			continue
		}
		R := glyph.Rows(c.want)
		y0 := c.h/2 - (R-1)/2
		minY, maxY, _ := focusRows(f)
		if minY != y0 || maxY != y0+R-1 {
			t.Errorf("size %d at %dx%d: focus rows %d..%d want %d..%d", c.size, c.w, c.h, minY, maxY, y0, y0+R-1)
		}
		if y0-1 < 1 || y0+R > c.h-2 {
			t.Errorf("size %d at %dx%d: word rows %d..%d leave no free rows", c.size, c.w, c.h, y0, y0+R-1)
		}
	}
	var got []int
	for _, d := range [][2]int{{80, 24}, {40, 8}, {19, 4}} {
		got = append(got, Render(Model{Word: "Hello", Size: 5}, d[0], d[1]).Size)
	}
	if got[0] != 5 || got[1] != 2 || got[2] != 0 {
		t.Errorf("AC14 sequence: %v", got)
	}
}

func TestGlyphFallback(t *testing.T) {
	const w, h = 80, 24
	words := []string{"Hello", "naïve", "¿Qué?", "ÆØÅ", "a", "x-y", "©2026"}
	for _, word := range words {
		for s := 2; s <= 5; s++ {
			f := Render(Model{Word: word, Size: s}, w, h)
			if f.Size != s {
				t.Errorf("%q size %d: drawn %d", word, s, f.Size)
				continue
			}
			R, W := glyph.Rows(s), glyph.Width(s)
			y0 := h/2 - (R-1)/2
			fx := w/2 - W/2
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					in := x >= fx && x < fx+W && y >= y0 && y < y0+R
					if (f.Cells[y][x].Style == StyleFocus) != in {
						t.Fatalf("%q size %d: focus mismatch at (%d,%d)", word, s, x, y)
					}
				}
			}
			fi := orp.Index(word)
			for k, c := range orp.Clusters(word) {
				rows, ok := glyph.Block([]rune(c)[0], s)
				if !ok {
					t.Fatalf("%q: no block for %q", word, c)
				}
				xk := w/2 - W/2 + (k-fi)*W
				for j := 0; j < R; j++ {
					rr := []rune(rows[j])
					for i := 0; i < W; i++ {
						want := ""
						if rr[i] != ' ' {
							want = string(rr[i])
						}
						if got := f.Cells[y0+j][xk+i].Text; got != want {
							t.Fatalf("%q size %d box %d (%d,%d): %q want %q", word, s, k, i, j, got, want)
						}
					}
				}
			}
		}
	}
	fallback := []string{"日本", "naïve", "€5", "Ωmega", "“quoted”", "\U0001F44D", "é"}
	for _, word := range fallback {
		for s := 2; s <= 5; s++ {
			f := Render(Model{Word: word, Size: s}, w, h)
			if f.Size != 1 {
				t.Errorf("%q size %d: drawn %d, want 1", word, s, f.Size)
			}
			_, _, n := focusRows(f)
			c := f.Cells[h/2][w/2]
			if n != 1 || c.Style != StyleFocus || c.Text != orp.Clusters(word)[orp.Index(word)] {
				t.Errorf("%q size %d: focus count %d cell %+v", word, s, n, c)
			}
		}
	}
}
