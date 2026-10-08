// Package tui renders and runs the terminal UI.
package tui

import (
	"github.com/0x6c6d/fastread/internal/orp"
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
	Cells [][]Cell
}

// Model is everything Render needs for one frame.
type Model struct {
	Word         string
	Size         int // 1..5 (rendered as 1 in this task)
	Paused       bool
	ShowProgress bool
	ShowHelp     bool
	Index, Total int // 0-based word index, word count
	WPM          int
	EffectiveWPM int // 0 = unknown
}

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
	row := f.Cells[h/2]
	put := func(x int, c string, cw int, st Style) {
		if x < 0 || x >= w {
			return
		}
		row[x] = Cell{Text: c, Style: st}
		if cw == 2 && x+1 < w {
			row[x+1] = Cell{Cont: true}
		}
	}
	cs := orp.Clusters(m.Word)
	if len(cs) == 0 {
		return f
	}
	fi := orp.Index(m.Word)
	fc := FocusColumn(w)
	// left of focus, walking backwards
	x := fc
	for i := fi - 1; i >= 0; i-- {
		cw := uniseg.StringWidth(cs[i])
		if cw <= 0 {
			continue
		}
		x -= cw
		put(x, cs[i], cw, StylePlain)
	}
	x = fc
	for i := fi; i < len(cs); i++ {
		cw := uniseg.StringWidth(cs[i])
		if cw <= 0 {
			continue
		}
		st := StylePlain
		if i == fi {
			st = StyleFocus
		}
		put(x, cs[i], cw, st)
		x += cw
	}
	return f
}
