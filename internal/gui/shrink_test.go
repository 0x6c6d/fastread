package gui

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/image/math/fixed"

	"github.com/0x6c6d/fastread/internal/orp"
)

func fitsRef(t *testing.T, m Measurer, text string, px fixed.Int26_6, w int) bool {
	t.Helper()
	cs := orp.Clusters(text)
	adv := m.Advances(cs, px)
	fi := orp.Index(text)
	var pre, total fixed.Int26_6
	for i, a := range adv {
		if i < fi {
			pre += a
		}
		total += a
	}
	o := fixed.I(w/2) - pre - adv[fi]/2
	return o >= 0 && o+total <= fixed.I(w)
}

func TestLayoutShrinkSplit(t *testing.T) {
	m := newFontMeasurer(t)
	w60 := strings.Repeat("abcdefghij", 6)
	don := "Donaudampfschifffahrtsgesellschaftskapitän"
	type row struct {
		word  string
		lvl   int
		w     int
		k     float32
		sp    int
		steps []string
	}
	var rows []row
	for l, sp := range []int{20, 32, 48, 64, 96} {
		rows = append(rows, row{"Hello", l + 1, 600, 1, sp, []string{"Hello"}})
	}
	rows = append(rows, row{"Hello", 5, 200, 1, 72, []string{"Hello"}})
	for l, sp := range []int{20, 22, 22, 22, 22} {
		rows = append(rows, row{"internationally", l + 1, 200, 1, sp, []string{"internationally"}})
	}
	rows = append(rows, row{"internationally", 5, 600, 1, 66, []string{"internationally"}})
	for l, sp := range []int{20, 32, 38, 38, 38} {
		rows = append(rows, row{w60, l + 1, 1920, 1, sp, []string{w60}})
	}
	for l := 1; l <= 5; l++ {
		rows = append(rows, row{w60, l, 1920, 2, 18, []string{w60}})
		rows = append(rows, row{w60, l, 600, 1, 10, []string{w60}})
		rows = append(rows, row{w60, l, 200, 1, 10, []string{"abcdefghijabcdefghijabcd-", "efghijabcdefghijabcdefghi-", "jabcdefghij"}})
		rows = append(rows, row{don, l, 200, 1, 10, []string{"Donaudampfschifffahrtsge-", "sellschaftskapitän"}})
	}
	for _, c := range rows {
		in := LayoutInput{Word: c.word, Level: c.lvl, W: c.w, H: 300, PxPerSp: c.k, M: m}
		r := Layout(in)
		if r.Sp != c.sp || strings.Join(r.Parts, "|") != strings.Join(c.steps, "|") {
			t.Errorf("%q L%d W%d k%v: Sp %d parts %q; want %d %q", c.word, c.lvl, c.w, c.k, r.Sp, r.Parts, c.sp, c.steps)
			continue
		}
		if c.k == 2 && r.Px != fixed.Int26_6(2304) {
			t.Errorf("Px %v", r.Px)
		}
		ref := Layout(LayoutInput{Word: "Hello", Level: c.lvl, W: c.w, H: 300, PxPerSp: c.k, M: m})
		var joined string
		for p, part := range r.Parts {
			in.Part = p
			rp := Layout(in)
			if rp.Text != part || rp.Sp != r.Sp {
				t.Errorf("step %d text %q", p, rp.Text)
			}
			if !fitsRef(t, m, part, rp.Px, c.w) {
				t.Errorf("%q step %d %q does not fit", c.word, p, part)
			}
			if rp.BaselineY != ref.BaselineY || rp.TickTop != ref.TickTop || rp.TickBottom != ref.TickBottom {
				t.Errorf("%q: baseline/ticks differ", c.word)
			}
			if p < len(r.Parts)-1 {
				if !strings.HasSuffix(part, "-") {
					t.Errorf("step %q lacks hyphen", part)
				}
				joined += strings.TrimSuffix(part, "-")
				// maximality: one more cluster plus "-", or the rest of the word
				rest := orp.Clusters(strings.Join(trimDashes(r.Parts[p:]), ""))
				k := len(orp.Clusters(part)) // clusters incl. dash => letters = k-1
				if k <= len(rest) {
					next := strings.Join(rest[:k], "")
					if k < len(rest) {
						next += "-"
					}
					if fitsRef(t, m, next, rp.Px, c.w) {
						t.Errorf("%q step %q not maximal", c.word, part)
					}
				}
			} else {
				joined += part
			}
		}
		if joined != c.word {
			t.Errorf("%q reassembles to %q", c.word, joined)
		}
		if r.Sp < LevelSp[c.lvl] {
			px := PxOf(r.Sp+2, c.k)
			if fitsRef(t, m, c.word, px, c.w) {
				t.Errorf("%q: Sp+2 fits", c.word)
			}
		}
		in.Part = -1
		if Layout(in).Text != r.Parts[0] {
			t.Errorf("part -1")
		}
		in.Part = 99
		if Layout(in).Text != r.Parts[len(r.Parts)-1] {
			t.Errorf("part 99")
		}
	}

	long := strings.Repeat("a", 100000)
	limit := 2 * time.Second
	if raceEnabled {
		limit = 20 * time.Second
	}
	start := time.Now()
	r := Layout(LayoutInput{Word: long, Level: 3, W: 600, H: 300, PxPerSp: 1, M: m})
	if d := time.Since(start); d > limit {
		t.Errorf("long word took %v", d)
	}
	if got := strings.ReplaceAll(strings.Join(r.Parts, ""), "-", ""); got != long {
		t.Errorf("long word reassembly failed (%d steps)", len(r.Parts))
	}
}

func trimDashes(ps []string) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		if i < len(ps)-1 {
			p = strings.TrimSuffix(p, "-")
		}
		out[i] = p
	}
	return out
}

func TestLayoutFocusXAllSizes(t *testing.T) {
	m := newFontMeasurer(t)
	words := strings.Fields(`a I ab Hello wonderful reading, "extraordinary" internationally (wonderful) -- — ... naïve ¿Qué? don't 1,000,000 2026 日本語の本 한국어 👍 Ωmega “quoted” ÆØÅ café end.`)
	words = append(words, "Donaudampfschifffahrtsgesellschaftskapitän", strings.Repeat("abcdefghij", 6), "state-of-the-art-technology-and-more", "x", "The", "resilience")
	cases := 0
	for _, word := range words {
		for l := 1; l <= 5; l++ {
			for _, w := range []int{200, 600, 601, 1920} {
				for _, k := range []float32{1, 2} {
					in := LayoutInput{Word: word, Level: l, W: w, H: 300, PxPerSp: k, M: m}
					n := len(Layout(in).Parts)
					for p := 0; p < n; p++ {
						in.Part = p
						r := Layout(in)
						cases++
						half := fixed.I(w / 2)
						if r.FocusX != half {
							t.Errorf("%q L%d W%d: FocusX %v", word, l, w, r.FocusX)
						}
						var pre fixed.Int26_6
						for _, a := range r.Advances[:r.Focus] {
							pre += a
						}
						if d := 2*(r.OriginX+pre) + r.Advances[r.Focus] - 2*half; d != 0 && d != 1 {
							t.Errorf("%q L%d W%d: centre error %d", word, l, w, d)
						}
						if r.Focus != orp.Index(r.Text) {
							t.Errorf("%q: focus", word)
						}
						if r.TickTop.Min.X != w/2-1 || r.TickTop.Max.X != w/2+1 || r.TickBottom.Min.X != w/2-1 || r.TickBottom.Max.X != w/2+1 {
							t.Errorf("%q: tick x", word)
						}
					}
				}
			}
		}
	}
	t.Logf("words=%d levels=5 widths=200,600,601,1920 cases=%d", len(words), cases)
}
