//go:build !nogui && cgo

package gui

import (
	"reflect"
	"testing"

	"github.com/0x6c6d/fastread/internal/orp"
)

func TestGioMeasurer(t *testing.T) {
	words := []string{
		"a", "I", "hello", "world,", "reading.", "fast!", "why?", "(quoted)", "\"say\"",
		"it's", "co-operate", "e-mail;", "50%", "1984", "$12.50", "x+y=z", "[note]",
		"{brace}", "path/to", "back\\slash", "under_score", "#tag", "@user", "&more",
		"*star*", "~tilde", "^caret", "`tick`", "|pipe|", "<angle>",
		"Ärger", "Übermut", "Österreich", "straße", "café", "naïve", "façade", "señor",
		"Ølstue", "Æsir", "þorn", "ðað", "œuvre", "¿qué?", "¡hola!", "«guillemets»",
		"£5", "¥100", "§12", "©2026", "®", "°C", "±3", "µm", "½", "¼¾", "×÷", "¬¦",
		"ÀÁÂÃÄÅ", "àáâãäå", "ÈÉÊËÌÍÎÏ", "ÒÓÔÕÖ", "ÙÚÛÜÝ", "ýÿ", "Ççñ", "WAVE", "fiflffi",
		"incomprehensibilities", "Donaudampfschifffahrt",
	}
	gm, err := newGioMeasurer()
	if err != nil {
		t.Fatal(err)
	}
	fm := newFontMeasurer(t)
	for _, sp := range []int{10, 20, 32, 48, 64, 96} {
		px := PxOf(sp, 1)
		if g, f := gm.Metrics(px), fm.Metrics(px); g != f {
			t.Errorf("Metrics(%v) = %+v, want %+v", px, g, f)
		}
		for _, w := range words {
			cs := orp.Clusters(w)
			if g, f := gm.Advances(cs, px), fm.Advances(cs, px); !reflect.DeepEqual(g, f) {
				t.Errorf("Advances(%q, %v) = %v, want %v", w, px, g, f)
			}
		}
	}
	for _, w := range words {
		for lvl := 1; lvl <= 5; lvl++ {
			for _, sz := range [][2]int{{600, 300}, {120, 80}, {1, 1}} {
				in := LayoutInput{Word: w, Level: lvl, W: sz[0], H: sz[1], PxPerSp: 1}
				in.M = gm
				g := Layout(in)
				in.M = fm
				f := Layout(in)
				if !reflect.DeepEqual(g, f) {
					t.Errorf("Layout(%q, level %d, %dx%d) differs:\n gio %+v\n  xi %+v", w, lvl, sz[0], sz[1], g, f)
				}
			}
		}
	}
}
