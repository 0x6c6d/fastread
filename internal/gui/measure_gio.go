//go:build !nogui && cgo

package gui

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

// goFont is the only font the GUI uses: the embedded Go Regular.
var goFont = font.Font{Typeface: "Go"}

// maxShapeCache bounds the shaped-glyph cache; it is cleared when it grows past this.
const maxShapeCache = 4096

type shapeKey struct {
	s  string
	px fixed.Int26_6
}

// shaped is one string shaped on its own at one size.
type shaped struct {
	glyphs  []text.Glyph
	advance fixed.Int26_6
}

// gioMeasurer implements Measurer with a single Gio text.Shaper over Go Regular (no system
// fonts). Shaped glyphs are cached per (string, px) and reused for drawing.
type gioMeasurer struct {
	sh    *text.Shaper
	cache map[shapeKey]shaped
}

func newGioMeasurer() (*gioMeasurer, error) {
	face, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("gui: font: %w", err)
	}
	sh := text.NewShaper(text.NoSystemFonts(), text.WithCollection([]font.FontFace{{Font: goFont, Face: face}}))
	return &gioMeasurer{sh: sh, cache: map[shapeKey]shaped{}}, nil
}

// shape returns the glyphs of s shaped as one line at px, from the cache when possible.
func (m *gioMeasurer) shape(s string, px fixed.Int26_6) shaped {
	k := shapeKey{s, px}
	if v, ok := m.cache[k]; ok {
		return v
	}
	if len(m.cache) >= maxShapeCache {
		clear(m.cache)
	}
	m.sh.LayoutString(text.Parameters{
		Font:             goFont,
		PxPerEm:          px,
		MaxWidth:         1 << 24,
		DisableSpaceTrim: true,
	}, s)
	var v shaped
	for {
		g, ok := m.sh.NextGlyph()
		if !ok {
			break
		}
		v.glyphs = append(v.glyphs, g)
		v.advance += g.Advance
	}
	m.cache[k] = v
	return v
}

// Advances shapes each cluster on its own; the advance is the sum of its glyph advances.
func (m *gioMeasurer) Advances(clusters []string, px fixed.Int26_6) []fixed.Int26_6 {
	out := make([]fixed.Int26_6, len(clusters))
	for i, c := range clusters {
		out[i] = m.shape(c, px).advance
	}
	return out
}

// Metrics are the ascent and descent of a shaped "H".
func (m *gioMeasurer) Metrics(px fixed.Int26_6) Metrics {
	v := m.shape("H", px)
	if len(v.glyphs) == 0 {
		return Metrics{}
	}
	g := v.glyphs[0]
	return Metrics{Ascent: g.Ascent, Descent: g.Descent}
}
