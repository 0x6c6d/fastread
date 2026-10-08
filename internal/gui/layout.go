package gui

import (
	"image"
	"math"

	"golang.org/x/image/math/fixed"

	"github.com/0x6c6d/fastread/internal/orp"
)

// LevelSp is the font size in sp per size level 1..5 (index 0 unused).
var LevelSp = [6]int{0, 20, 32, 48, 64, 96}

const (
	MinSp        = 10
	ShrinkStepSp = 2
	TickGapPx    = 4
	TickWidthPx  = 2
)

// Metrics are the vertical font metrics in pixels; both are positive.
type Metrics struct{ Ascent, Descent fixed.Int26_6 }

// Measurer measures grapheme clusters in the embedded Go Regular font.
type Measurer interface {
	Advances(clusters []string, px fixed.Int26_6) []fixed.Int26_6
	Metrics(px fixed.Int26_6) Metrics
}

// LayoutInput is the input of Layout.
type LayoutInput struct {
	Word    string
	Level   int // clamped to 1..5
	W, H    int // window px; values < 1 count as 1
	PxPerSp float32
	Part    int // display step, clamped to [0, len(Parts)-1]
	M       Measurer
}

// LayoutResult is the pure layout of one word.
type LayoutResult struct {
	CX                  int // W/2, integer division
	Sp                  int
	Px                  fixed.Int26_6
	Parts               []string
	Part                int
	Text                string
	Clusters            []string
	Advances            []fixed.Int26_6
	Focus               int
	OriginX             fixed.Int26_6 // pen x of the first cluster
	FocusX              fixed.Int26_6 // centre of the focus cluster
	BaselineY           fixed.Int26_6
	TickTop, TickBottom image.Rectangle
}

// PxOf converts sp to 26.6 pixels, rounded to nearest:
// Int26_6(math.Round(sp*pxPerSp*64)). pxPerSp <= 0 counts as 1.
func PxOf(sp int, pxPerSp float32) fixed.Int26_6 {
	if !(pxPerSp > 0) {
		pxPerSp = 1
	}
	return fixed.Int26_6(math.Round(float64(sp) * float64(pxPerSp) * 64))
}

// Layout computes the focus x, baseline and ticks. Rounding: OriginX uses Go
// integer division on 26.6 units for fa/2 and (Ascent-Descent)/2; the tick
// baseline uses Round, ascent/descent use Ceil. It never panics.
func Layout(in LayoutInput) LayoutResult {
	l := min(max(in.Level, 1), 5)
	w, h := max(in.W, 1), max(in.H, 1)
	k := in.PxPerSp
	if !(k > 0) {
		k = 1
	}
	var r LayoutResult
	r.CX = w / 2
	r.Sp = LevelSp[l]
	r.Parts = splitParts(in.Word)
	r.Part = min(max(in.Part, 0), len(r.Parts)-1)
	r.Text = r.Parts[r.Part]
	r.Px = PxOf(r.Sp, k)
	r.Clusters = orp.Clusters(r.Text)
	r.Advances = in.M.Advances(r.Clusters, r.Px)
	r.Focus = orp.Index(r.Text)

	var prefix, fa fixed.Int26_6
	for i, a := range r.Advances {
		if i < r.Focus {
			prefix += a
		}
	}
	if r.Focus >= 0 && r.Focus < len(r.Advances) {
		fa = r.Advances[r.Focus]
	}
	r.OriginX = fixed.I(r.CX) - prefix - fa/2
	r.FocusX = r.OriginX + prefix + fa/2

	mt := in.M.Metrics(PxOf(LevelSp[l], k))
	r.BaselineY = fixed.I(h/2) + (mt.Ascent-mt.Descent)/2
	base := r.BaselineY.Round()
	a, d := mt.Ascent.Ceil(), mt.Descent.Ceil()
	tl := max(2, a/3)
	r.TickTop = image.Rect(r.CX-1, base-a-TickGapPx-tl, r.CX+1, base-a-TickGapPx)
	r.TickBottom = image.Rect(r.CX-1, base+d+TickGapPx, r.CX+1, base+d+TickGapPx+tl)
	return r
}

// splitParts is the single replaceable spot for shrink/split.
func splitParts(word string) []string { return []string{word} }
