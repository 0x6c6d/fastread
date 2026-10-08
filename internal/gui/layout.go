package gui

import (
	"image"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

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
	r.Sp, r.Parts = shrinkSplit(in.Word, l, w, k, in.M)
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

// rank5 maps the number of letter/digit clusters to the R9 table value.
func rank5(l int) int {
	switch {
	case l <= 1:
		return 0
	case l <= 5:
		return 1
	case l <= 9:
		return 2
	case l <= 13:
		return 3
	default:
		return 4
	}
}

func isLetterDigitCluster(c string) bool {
	r, _ := utf8.DecodeRuneInString(c)
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// prefixSums returns p with p[i] = sum(a[:i]).
func prefixSums(a []fixed.Int26_6) []fixed.Int26_6 {
	p := make([]fixed.Int26_6, len(a)+1)
	for i, v := range a {
		p[i+1] = p[i] + v
	}
	return p
}

// fitsAt reports the R23 fit rule for a text with advances adv: the focus
// cluster fi is centred at cx and the whole text stays inside [0, w].
func fitsAt(adv []fixed.Int26_6, fi, cx, w int) bool {
	if len(adv) == 0 {
		return true
	}
	var pre, total fixed.Int26_6
	for i, a := range adv {
		if i < fi {
			pre += a
		}
		total += a
	}
	o := fixed.I(cx) - pre - adv[fi]/2
	return o >= 0 && o+total <= fixed.I(w)
}

// shrinkSplit implements R23: shrink in ShrinkStepSp steps down to MinSp, then
// split greedily at MinSp. It returns the size in sp and the display steps.
func shrinkSplit(word string, level, w int, k float32, m Measurer) (int, []string) {
	cx := w / 2
	cs := orp.Clusters(word)
	n := len(cs)
	if n == 0 {
		return LevelSp[level], []string{word}
	}
	whole := func(sp int) bool {
		adv := m.Advances(cs, PxOf(sp, k))
		return fitsAt(adv, orp.Index(word), cx, w)
	}
	if whole(MinSp) {
		for sp := LevelSp[level]; sp > MinSp; sp -= ShrinkStepSp {
			if whole(sp) {
				return sp, []string{word}
			}
		}
		return MinSp, []string{word}
	}

	px := PxOf(MinSp, k)
	adv := m.Advances(cs, px)
	da := m.Advances([]string{"-"}, px)[0]
	pre := prefixSums(adv)
	// cnt[i] = letter/digit clusters before i; lpos = their indices.
	cnt := make([]int, n+1)
	var lpos []int
	for i, c := range cs {
		cnt[i+1] = cnt[i]
		if isLetterDigitCluster(c) {
			cnt[i+1]++
			lpos = append(lpos, i)
		}
	}
	// fits reports whether clusters i..j-1 (plus "-" when j < n) fit.
	fits := func(i, j int) bool {
		dash := j < n
		cnum := j - i
		if dash {
			cnum++
		}
		var fi int // absolute index; == j means the dash
		if L := cnt[j] - cnt[i]; L == 0 {
			fi = i + (cnum-1)/2
		} else {
			fi = lpos[cnt[i]+rank5(L)]
		}
		fa := da
		if fi < n && fi < j {
			fa = adv[fi]
		}
		total := pre[j] - pre[i]
		if dash {
			total += da
		}
		o := fixed.I(cx) - (pre[fi] - pre[i]) - fa/2
		return o >= 0 && o+total <= fixed.I(w)
	}
	var parts []string
	for i := 0; i < n; {
		j := i + 1
		for j < n && fits(i, j+1) {
			j++
		}
		p := strings.Join(cs[i:j], "")
		if j < n {
			p += "-"
		}
		parts = append(parts, p)
		i = j
	}
	return MinSp, parts
}
