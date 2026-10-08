// Package tui renders and runs the terminal UI.
package tui

import (
	"github.com/0x6c6d/fastread/internal/orp"
	"github.com/0x6c6d/fastread/internal/tui/glyph"
	"github.com/rivo/uniseg"
)

// Style is the visual style of a cell.
type Style uint8

const (
	StylePlain Style = iota // white letters, blank cells
	StyleFocus              // the red focus cluster
	StyleTick               // guide tick (used by a later task)
)

// Cell is one terminal cell. Text is a grapheme cluster or "" for a blank cell.
// Cont is true for the second cell of a 2-cell-wide cluster (Encode writes nothing for it).
type Cell struct {
	Text  string
	Style Style
	Cont  bool
}

// Frame is a W x H grid; Cells[y][x], len(Cells) == H, len(Cells[y]) == W.
type Frame struct {
	W, H  int
	Size  int // size level actually drawn; 0 when nothing is drawn
	Cells [][]Cell
}

// Model is everything Render needs for one frame.
type Model struct {
	Word         string
	Size         int // 1..5
	Paused       bool
	ShowProgress bool
	ShowHelp     bool
	Index, Total int // 0-based word index, word count
	WPM          int
	EffectiveWPM int // 0 = unknown
}

// TooSmallText is shown when the terminal is too small to draw a word.
const TooSmallText = "terminal too small"

// FocusColumn returns the fixed focus column for width w: w/2.
func FocusColumn(w int) int { return w / 2 }

// Render draws m into a w x h frame. It never panics; cells outside the frame are clipped.
func Render(m Model, w, h int) Frame {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	f := Frame{W: w, H: h, Cells: make([][]Cell, h)}
	for y := range f.Cells {
		f.Cells[y] = make([]Cell, w)
	}
	if w == 0 || h == 0 {
		return f
	}
	s := EffectiveSize(m.Word, m.Size, w, h)
	f.Size = s
	if s == 0 {
		x0 := w - len(TooSmallText)
		if x0 < 0 {
			x0 = 0
		} else {
			x0 /= 2
		}
		for i, r := range []rune(TooSmallText) {
			if x0+i >= w {
				break
			}
			f.Cells[h/2][x0+i] = Cell{Text: string(r)}
		}
		return f
	}
	R := glyph.Rows(s)
	y0 := h/2 - (R-1)/2
	for _, ty := range []int{y0 - 1, y0 + R} {
		if ty >= 0 && ty < h {
			f.Cells[ty][w/2] = Cell{Text: "│", Style: StyleTick}
		}
	}
	if s >= 2 {
		renderBlocks(&f, m.Word, s)
		return f
	}
	row := f.Cells[h/2]
	cs := orp.Clusters(m.Word)
	if len(cs) == 0 {
		return f
	}
	fi := orp.Index(m.Word)
	fc := FocusColumn(w)
	// Cell width of each cluster; zero-width clusters take one cell, drawn after U+25CC.
	texts := make([]string, len(cs))
	cws := make([]int, len(cs))
	for i, c := range cs {
		texts[i] = c
		cws[i] = uniseg.StringWidth(c)
		if cws[i] <= 0 {
			texts[i] = "\u25cc" + c
			cws[i] = 1
		}
	}
	starts := make([]int, len(cs))
	starts[fi] = fc
	for k := fi + 1; k < len(cs); k++ {
		starts[k] = starts[k-1] + cws[k-1]
	}
	for k := fi - 1; k >= 0; k-- {
		starts[k] = starts[k+1] - cws[k]
	}
	for k := range cs {
		x, cw := starts[k], cws[k]
		if x < 0 || x+cw > w {
			continue // never draw part of a cluster
		}
		st := StylePlain
		if k == fi {
			st = StyleFocus
		}
		row[x] = Cell{Text: texts[k], Style: st}
		if cw == 2 {
			row[x+1] = Cell{Cont: true}
		}
	}
	return f
}

// renderBlocks draws word as block glyphs of size s (>= 2); all clusters have glyphs.
func renderBlocks(f *Frame, word string, s int) {
	cs := orp.Clusters(word)
	if len(cs) == 0 {
		return
	}
	fi := orp.Index(word)
	R, W := glyph.Rows(s), glyph.Width(s)
	y0 := f.H/2 - (R-1)/2
	for k, c := range cs {
		rows, ok := glyph.Block([]rune(c)[0], s)
		if !ok {
			continue
		}
		xk := f.W/2 - W/2 + (k-fi)*W
		for j := 0; j < R; j++ {
			y := y0 + j
			if y < 0 || y >= f.H {
				continue
			}
			rr := []rune(rows[j])
			for i := 0; i < W; i++ {
				x := xk + i
				if x < 0 || x >= f.W {
					continue
				}
				st := StylePlain
				if k == fi {
					st = StyleFocus
				}
				cell := Cell{Style: st}
				if i < len(rr) && rr[i] != ' ' {
					cell.Text = string(rr[i])
				}
				f.Cells[y][x] = cell
			}
		}
	}
}
