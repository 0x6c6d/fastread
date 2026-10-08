package gui

import (
	"fmt"
	"image"
	"reflect"
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/tui"
)

func TestLayoutChrome(t *testing.T) {
	fm := newFontMeasurer(t)
	if HelpText != tui.HelpText {
		t.Errorf("HelpText differs from tui.HelpText")
	}
	rows := []struct {
		index, total int
		fill         image.Rectangle
	}{
		{4, 120, image.Rect(8, 267, 32, 269)},
		{0, 3, image.Rect(8, 267, 202, 269)},
		{119, 120, image.Rect(8, 267, 592, 269)},
		{4, 0, image.Rect(8, 267, 8, 269)},
	}
	for _, r := range rows {
		c := LayoutChrome(600, 300, 1, r.index, r.total, fm)
		if c.Help != image.Pt(8, 22) || c.Progress != image.Pt(8, 289) {
			t.Errorf("%d/%d: help %v progress %v", r.index, r.total, c.Help, c.Progress)
		}
		if c.Bar != image.Rect(8, 267, 592, 269) {
			t.Errorf("%d/%d: bar %v", r.index, r.total, c.Bar)
		}
		if c.BarFill != r.fill {
			t.Errorf("%d/%d: fill %v want %v", r.index, r.total, c.BarFill, r.fill)
		}
	}

	pt := []struct {
		i, n, wpm int
		ok        bool
		want      string
	}{
		{4, 120, 287, true, "word 5/120  287 wpm"},
		{0, 3, 0, false, "word 1/3  — wpm"},
		{0, 3, 300, false, "word 1/3  — wpm"},
		{0, 3, 0, true, "word 1/3  — wpm"},
	}
	for _, r := range pt {
		if got := ProgressText(r.i, r.n, r.wpm, r.ok); got != r.want {
			t.Errorf("ProgressText = %q want %q", got, r.want)
		}
	}

	sizes := []struct {
		w, h int
		k    float32
	}{{600, 300, 1}, {801, 401, 1}, {1920, 1080, 1}, {801, 401, 1.5}, {1920, 1080, 1.5}}
	words := []string{"Hello", strings.Repeat("abcdefghij", 6)}
	for _, s := range sizes {
		for _, word := range words {
			for lvl := 1; lvl <= 5; lvl++ {
				name := fmt.Sprintf("%dx%d k%v L%d %.5s", s.w, s.h, s.k, lvl, word)
				in := LayoutInput{Word: word, Level: lvl, W: s.w, H: s.h, PxPerSp: s.k, M: fm}
				lr := Layout(in)
				c := LayoutChrome(s.w, s.h, s.k, 3, 10, fm)
				cm := fm.Metrics(c.Px)
				d := cm.Descent.Ceil()
				if c.Help.Y+d > lr.TickTop.Min.Y {
					t.Errorf("%s: help %d+%d overlaps top tick %v", name, c.Help.Y, d, lr.TickTop)
				}
				if c.Bar.Min.Y < lr.TickBottom.Max.Y {
					t.Errorf("%s: bar %v overlaps bottom tick %v", name, c.Bar, lr.TickBottom)
				}
				wm := fm.Metrics(PxOf(LevelSp[lvl], s.k))
				base := lr.BaselineY.Round()
				top, bot := base-wm.Ascent.Ceil(), base+wm.Descent.Ceil()
				if top < lr.TickTop.Max.Y || bot > lr.TickBottom.Min.Y {
					t.Errorf("%s: word box %d..%d outside ticks %v %v", name, top, bot, lr.TickTop, lr.TickBottom)
				}
				for _, it := range [][2]int{{0, 0}, {99, 5}, {0, -1}} {
					LayoutChrome(s.w, s.h, s.k, it[0], it[1], fm)
					if !reflect.DeepEqual(lr, Layout(in)) {
						t.Errorf("%s: Layout not stable", name)
					}
				}
			}
		}
	}

	for _, w := range []int{0, 1, 600} {
		for _, h := range []int{0, 1, 300} {
			for _, k := range []float32{0, -1, 1} {
				LayoutChrome(w, h, k, 9, -1, fm)
				LayoutChrome(w, h, k, 9, 3, fm)
				LayoutChrome(w, h, k, -5, 3, fm)
			}
		}
	}
}
