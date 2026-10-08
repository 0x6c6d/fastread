package gui

import (
	"fmt"
	"image"

	"golang.org/x/image/math/fixed"
)

// HelpText is the GUI help line (R28); identical to the TUI's.
const HelpText = "space pause  ↑↓ wpm  [ ] size  ←→ word  home restart  p progress  ? help  q quit"

const (
	ChromeSp       = 14
	ChromeMarginPx = 8
	BarHeightPx    = 2
	BarGapPx       = 6
)

// Chrome is where the help line and the progress bar/text go (R27, R28). It never depends
// on the word, so toggling help or progress can never move the focus.
type Chrome struct {
	Px           fixed.Int26_6   // PxOf(ChromeSp, pxPerSp): text size of both lines
	Help         image.Point     // left end of the help line's baseline
	Progress     image.Point     // left end of the progress text's baseline
	Bar, BarFill image.Rectangle // whole bar and its filled part
}

// LayoutChrome computes the help and progress geometry. It never panics.
func LayoutChrome(w, h int, pxPerSp float32, index, total int, m Measurer) Chrome {
	w, h = max(w, 1), max(h, 1)
	var c Chrome
	c.Px = PxOf(ChromeSp, pxPerSp)
	mt := m.Metrics(c.Px)
	a, d := mt.Ascent.Ceil(), mt.Descent.Ceil()
	M := ChromeMarginPx
	c.Help = image.Pt(M, M+a)
	c.Progress = image.Pt(M, h-M-d)
	y1 := c.Progress.Y - a - BarGapPx
	y0 := y1 - BarHeightPx
	bw := max(0, w-2*M)
	c.Bar = image.Rect(M, y0, M+bw, y1)
	fill := 0
	if total > 0 {
		fill = min(max((index+1)*bw/total, 0), bw)
	}
	c.BarFill = image.Rect(M, y0, M+fill, y1)
	return c
}

// ProgressText formats the progress text like the TUI: "word %d/%d  %s wpm" with index+1,
// total and the effective wpm, or "—" (U+2014) when !ok or wpm <= 0.
func ProgressText(index, total, wpm int, ok bool) string {
	e := "—"
	if ok && wpm > 0 {
		e = fmt.Sprint(wpm)
	}
	return fmt.Sprintf("word %d/%d  %s wpm", index+1, total, e)
}
