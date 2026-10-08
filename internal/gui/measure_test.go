package gui

import (
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type fontMeasurer struct {
	t     testing.TB
	f     *opentype.Font
	faces map[fixed.Int26_6]font.Face
}

func (m *fontMeasurer) face(px fixed.Int26_6) font.Face {
	if f, ok := m.faces[px]; ok {
		return f
	}
	f, err := opentype.NewFace(m.f, &opentype.FaceOptions{Size: float64(px) / 64, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		m.t.Fatal(err)
	}
	m.faces[px] = f
	return f
}

func (m *fontMeasurer) Advances(clusters []string, px fixed.Int26_6) []fixed.Int26_6 {
	face := m.face(px)
	out := make([]fixed.Int26_6, len(clusters))
	for i, c := range clusters {
		for _, r := range c {
			a, _ := face.GlyphAdvance(r)
			out[i] += a
		}
	}
	return out
}

func (m *fontMeasurer) Metrics(px fixed.Int26_6) Metrics {
	fm := m.face(px).Metrics()
	return Metrics{Ascent: fm.Ascent, Descent: fm.Descent}
}

func newFontMeasurer(t testing.TB) Measurer {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return &fontMeasurer{t: t, f: f, faces: map[fixed.Int26_6]font.Face{}}
}

type fixedMeasurer struct{ over map[string]fixed.Int26_6 }

func (m fixedMeasurer) Advances(clusters []string, px fixed.Int26_6) []fixed.Int26_6 {
	out := make([]fixed.Int26_6, len(clusters))
	for i, c := range clusters {
		if v, ok := m.over[c]; ok {
			out[i] = v
		} else {
			out[i] = px / 2
		}
	}
	return out
}

func (m fixedMeasurer) Metrics(px fixed.Int26_6) Metrics {
	return Metrics{Ascent: px * 3 / 4, Descent: px / 4}
}
