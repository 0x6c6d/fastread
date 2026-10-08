package gui

import (
	"image"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/image/math/fixed"

	"github.com/0x6c6d/fastread/internal/orp"
)

var WL = []string{
	"a", "I", "ab", "Hello", "wonderful", "reading,", `"extraordinary"`, "internationally",
	"(wonderful)", "--", "—", "...", "naïve", "¿Qué?", "don't", "1,000,000", "2026",
	"日本語の本", "한국어", "👍", "Ωmega", "“quoted”", "ÆØÅ", "café", "end.",
	"Donaudampfschifffahrtsgesellschaftskapitän", strings.Repeat("abcdefghij", 6),
	"x", "The", "of", "and", "resilience",
}

func checkRow(t *testing.T, in LayoutInput) LayoutResult {
	t.Helper()
	r := Layout(in)
	if !reflect.DeepEqual(r.Advances, in.M.Advances(orp.Clusters(r.Text), r.Px)) {
		t.Errorf("%q: advances mismatch", in.Word)
	}
	if r.Focus != orp.Index(r.Text) {
		t.Errorf("%q: focus %d", in.Word, r.Focus)
	}
	cx := fixed.I(max(in.W, 1) / 2)
	if r.FocusX != cx {
		t.Errorf("%q: FocusX %d want %d", in.Word, r.FocusX, cx)
	}
	var prefix, fa fixed.Int26_6
	for i := 0; i < r.Focus && i < len(r.Advances); i++ {
		prefix += r.Advances[i]
	}
	if r.Focus < len(r.Advances) {
		fa = r.Advances[r.Focus]
	}
	if d := 2*(r.OriginX+prefix) + fa - 2*cx; d != 0 && d != 1 {
		t.Errorf("%q: centre error %d", in.Word, d)
	}
	return r
}

func TestLayoutFocusX(t *testing.T) {
	fm := newFontMeasurer(t)
	rows := []struct {
		word  string
		lvl   int
		w     int
		k     float32
		focus int
		org   fixed.Int26_6
		px    fixed.Int26_6
	}{
		{"Hello", 1, 600, 1, 1, 17920, 0},
		{"Hello", 2, 600, 1, 1, 17152, 0},
		{"Hello", 3, 600, 1, 1, 16127, 0},
		{"Hello", 4, 600, 1, 1, 15103, 0},
		{"Hello", 5, 600, 1, 1, 13055, 0},
		{"Hello", 2, 601, 1, 1, 17152, 0},
		{"Hello", 2, 600, 1.5, 1, 16127, 3072},
		{"wonderful", 3, 600, 1, 2, 14418, 0},
		{"(wonderful)", 3, 600, 1, 3, 13395, 0},
		{"--", 2, 600, 1, 0, 18602, 0},
		{"", 2, 600, 1, 0, 19200, 0},
	}
	for _, c := range rows {
		r := checkRow(t, LayoutInput{Word: c.word, Level: c.lvl, W: c.w, H: 300, PxPerSp: c.k, M: fm})
		if c.word != "" && r.Focus != c.focus {
			t.Errorf("%q L%d: focus %d want %d", c.word, c.lvl, r.Focus, c.focus)
		}
		if r.OriginX != c.org {
			t.Errorf("%q L%d W%d k%v: origin %d want %d", c.word, c.lvl, c.w, c.k, r.OriginX, c.org)
		}
		if c.px != 0 && r.Px != c.px {
			t.Errorf("%q: px %d want %d", c.word, r.Px, c.px)
		}
	}
	r := checkRow(t, LayoutInput{Word: "abc", Level: 1, W: 100, H: 100, PxPerSp: 1, M: fixedMeasurer{}})
	if r.Focus != 1 || r.OriginX != fixed.I(35) {
		t.Errorf("abc fixed: %d %d", r.Focus, r.OriginX)
	}
	r = checkRow(t, LayoutInput{Word: "abc", Level: 1, W: 100, H: 100, PxPerSp: 1,
		M: fixedMeasurer{over: map[string]fixed.Int26_6{"b": 1139}}})
	if r.OriginX != 1991 || r.FocusX != 3200 {
		t.Errorf("abc override: %d %d", r.OriginX, r.FocusX)
	}
	// No-panic and clamping rows.
	base := LayoutInput{Word: "Hello", Level: 1, W: 600, H: 300, PxPerSp: 1, M: fm}
	for _, mod := range []func(*LayoutInput){
		func(i *LayoutInput) { i.W = 0 }, func(i *LayoutInput) { i.W = -5 },
		func(i *LayoutInput) { i.H = 0 }, func(i *LayoutInput) { i.Level = 0 },
		func(i *LayoutInput) { i.Level = 9 }, func(i *LayoutInput) { i.PxPerSp = 0 },
		func(i *LayoutInput) { i.Word = "" }, func(i *LayoutInput) { i.Part = 7 },
	} {
		in := base
		mod(&in)
		checkRow(t, in)
	}
	eq := func(a, b LayoutInput) {
		t.Helper()
		if !reflect.DeepEqual(Layout(a), Layout(b)) {
			t.Errorf("clamp mismatch: %+v vs %+v", a, b)
		}
	}
	a, b := base, base
	a.Level = 0
	eq(a, base)
	a.Level, b.Level = 9, 5
	eq(a, b)
	a, b = base, base
	a.PxPerSp = 0
	eq(a, base)
	for _, w := range WL {
		for lvl := 1; lvl <= 5; lvl++ {
			for _, W := range []int{200, 600, 601, 1920} {
				checkRow(t, LayoutInput{Word: w, Level: lvl, W: W, H: 300, PxPerSp: 1, M: fm})
			}
		}
	}
}

func TestLayoutBaseline(t *testing.T) {
	fm := newFontMeasurer(t)
	want := []fixed.Int26_6{0, 10069, 10351, 10727, 11103, 11854}
	for lvl := 1; lvl <= 5; lvl++ {
		r := Layout(LayoutInput{Word: "Hello", Level: lvl, W: 600, H: 300, PxPerSp: 1, M: fm})
		if r.BaselineY != want[lvl] {
			t.Errorf("L%d baseline %d want %d", lvl, r.BaselineY, want[lvl])
		}
	}
	if r := Layout(LayoutInput{Word: "Hello", Level: 2, W: 600, H: 300, PxPerSp: 1.5, M: fm}); r.BaselineY != 10727 {
		t.Errorf("L2 k1.5 baseline %d", r.BaselineY)
	}
	for lvl := 1; lvl <= 5; lvl++ {
		for _, h := range []int{300, 301, 1080} {
			for _, k := range []float32{1, 1.5} {
				mt := fm.Metrics(PxOf(LevelSp[lvl], k))
				rule := fixed.I(h/2) + (mt.Ascent-mt.Descent)/2
				for _, w := range WL {
					for _, W := range []int{200, 600, 1920} {
						r := Layout(LayoutInput{Word: w, Level: lvl, W: W, H: h, PxPerSp: k, M: fm})
						if r.BaselineY != rule {
							t.Fatalf("%q L%d H%d k%v W%d: %d want %d", w, lvl, h, k, W, r.BaselineY, rule)
						}
					}
				}
			}
		}
	}
}

func TestLayoutTicks(t *testing.T) {
	fm := newFontMeasurer(t)
	rc := image.Rect
	rows := []struct{ top, bot image.Rectangle }{
		{},
		{rc(299, 128, 301, 134), rc(299, 166, 301, 172)},
		{rc(299, 117, 301, 127), rc(299, 173, 301, 183)},
		{rc(299, 103, 301, 118), rc(299, 183, 301, 198)},
		{rc(299, 88, 301, 108), rc(299, 191, 301, 211)},
		{rc(299, 60, 301, 90), rc(299, 210, 301, 240)},
	}
	for lvl := 1; lvl <= 5; lvl++ {
		for _, W := range []int{600, 601} {
			r := Layout(LayoutInput{Word: "Hello", Level: lvl, W: W, H: 300, PxPerSp: 1, M: fm})
			if r.TickTop != rows[lvl].top || r.TickBottom != rows[lvl].bot {
				t.Errorf("L%d W%d: %v %v want %v %v", lvl, W, r.TickTop, r.TickBottom, rows[lvl].top, rows[lvl].bot)
			}
		}
	}
	r := Layout(LayoutInput{Word: "abc", Level: 1, W: 100, H: 100, PxPerSp: 1, M: fixedMeasurer{}})
	if r.TickTop != rc(49, 31, 51, 36) || r.TickBottom != rc(49, 64, 51, 69) {
		t.Errorf("fixed ticks %v %v", r.TickTop, r.TickBottom)
	}
	for lvl := 1; lvl <= 5; lvl++ {
		for _, W := range []int{200, 600, 601, 1920} {
			var first LayoutResult
			for i, w := range WL {
				r := Layout(LayoutInput{Word: w, Level: lvl, W: W, H: 300, PxPerSp: 1, M: fm})
				if i == 0 {
					first = r
				}
				if r.TickTop != first.TickTop || r.TickBottom != first.TickBottom {
					t.Fatalf("%q ticks differ", w)
				}
				mt := fm.Metrics(PxOf(LevelSp[lvl], 1))
				base := r.BaselineY.Round()
				for _, tk := range []image.Rectangle{r.TickTop, r.TickBottom} {
					if tk.Dx() != TickWidthPx || (tk.Min.X+tk.Max.X)/2 != r.CX || tk.Dy() < 2 {
						t.Fatalf("bad tick %v", tk)
					}
				}
				if r.TickTop.Max.Y != base-mt.Ascent.Ceil()-TickGapPx || r.TickBottom.Min.Y != base+mt.Descent.Ceil()+TickGapPx {
					t.Fatalf("tick position %v %v", r.TickTop, r.TickBottom)
				}
			}
		}
	}
}
