package tui

import (
	"github.com/0x6c6d/fastread/internal/orp"
	"github.com/0x6c6d/fastread/internal/tui/glyph"
)

// EffectiveSize returns the size level Render uses for word at w x h:
// 0 if w < 20 or h < 5; otherwise s := clamp(size, 1, 5), decreased while s >= 2 and
// the glyph rows (plus 4 free rows) or two glyph widths do not fit; then 1 if s >= 2 and
// some grapheme cluster of word is not exactly one rune with a block glyph; else s.
func EffectiveSize(word string, size, w, h int) int {
	if w < 20 || h < 5 {
		return 0
	}
	s := size
	if s < 1 {
		s = 1
	}
	if s > 5 {
		s = 5
	}
	for s >= 2 && !(glyph.Rows(s)+4 <= h && w-w/2 >= 2*glyph.Width(s)-glyph.Width(s)/2) {
		s--
	}
	if s >= 2 {
		for _, c := range orp.Clusters(word) {
			rs := []rune(c)
			if len(rs) != 1 || !glyph.Has(rs[0]) {
				return 1
			}
		}
	}
	return s
}
