// Package tui renders and runs the terminal UI.
package tui

import (
	"fmt"
	"strconv"

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
	Parts int // number of display steps of the word; 0 when nothing is drawn
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
	Part         int // 0-based display step of a long word
}

// TooSmallText is shown when the terminal is too small to draw a word.
const TooSmallText = "terminal too small"

// FocusColumn returns the fixed focus column for width w: w/2.
func FocusColumn(w int) int { return w / 2 }

// Render draws m into a w x h frame. It never panics; cells outside the frame are clipped.
func Render(m Model, w, h int) Frame {
	f := renderWord(m, w, h)
	if f.Size >= 1 {
		addRows(&f, m)
	}
	return f
}

// HelpText is the help row shown with ShowHelp.
const HelpText = "space pause  ↑↓ wpm  [ ] size  ←→ word  home restart  p progress  ? help  q quit"

// addRows draws the help row (row 0) and the progress row (row h-1).
func addRows(f *Frame, m Model) {
	put := func(y, x int, r rune) {
		if x >= 0 && x < f.W {
			f.Cells[y][x] = Cell{Text: string(r)}
		}
	}
	if m.ShowHelp {
		for i, r := range []rune(HelpText) {
			put(0, i, r)
		}
	}
	if !m.ShowProgress {
		return
	}
	y := f.H - 1
	e := "\u2014"
	if m.EffectiveWPM > 0 {
		e = strconv.Itoa(m.EffectiveWPM)
	}
	text := []rune(fmt.Sprintf("word %d/%d  %s wpm", m.Index+1, m.Total, e))
	tw := len(text)
	start := 0
	if f.W >= tw+2 {
		bw := f.W - tw - 1
		filled := 0
		if m.Total > 0 {
			filled = (m.Index + 1) * bw / m.Total
		}
		if filled < 0 {
			filled = 0
		}
		if filled > bw {
			filled = bw
		}
		for x := 0; x < bw; x++ {
			if x < filled {
				put(y, x, '\u2501')
			} else {
				put(y, x, '\u2500')
			}
		}
		start = bw + 1
	}
	for i, r := range text {
		put(y, start+i, r)
	}
}

func renderWord(m Model, w, h int) Frame {
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
	parts := Split(m.Word, s, w)
	p := m.Part
	if p > len(parts)-1 {
		p = len(parts) - 1
	}
	if p < 0 {
		p = 0
	}
	f.Parts = len(parts)
	word := parts[p]
	R := glyph.Rows(s)
	y0 := h/2 - (R-1)/2
	for _, ty := range []int{y0 - 1, y0 + R} {
		if ty >= 0 && ty < h {
			f.Cells[ty][w/2] = Cell{Text: "│", Style: StyleTick}
		}
	}
	if s >= 2 {
		renderBlocks(&f, word, s)
		return f
	}
	row := f.Cells[h/2]
	cs := orp.Clusters(word)
	if len(cs) == 0 {
		return f
	}
	fi := orp.Index(word)
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
