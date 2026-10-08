package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/0x6c6d/fastread/internal/orp"
	"github.com/0x6c6d/fastread/internal/tui/glyph"
	"github.com/rivo/uniseg"
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

// Split returns the display steps of word at size level size (1..5, from EffectiveSize)
// and width w. A text t fits iff, with fi = orp.Index(t) over its clusters cs:
// level 1: sum(width cs[:fi]) <= w/2 and w/2 + sum(width cs[fi:]) <= w;
// size >= 2: fi*W <= w/2-W/2 and w/2-W/2 + (len(cs)-fi)*W <= w.
// Greedy over the word's clusters c[0..n): from i, j := i+1; while j < n and
// cand(j+1) fits, j++; emit cand(j); i = j. cand(j) = join(c[i:j]) + "-" if j < n,
// else join(c[i:j]). A word that fits is one step equal to the word; "" → [""].
func Split(word string, size, w int) []string {
	cs := orp.Clusters(word)
	n := len(cs)
	if n == 0 || size < 1 {
		return []string{word}
	}
	cw := make([]int, n)
	pw := make([]int, n+1) // prefix widths
	pl := make([]int, n+1) // prefix letter/digit counts
	var lets []int
	for k, c := range cs {
		if size >= 2 {
			cw[k] = glyph.Width(size)
		} else if cw[k] = uniseg.StringWidth(c); cw[k] < 1 {
			cw[k] = 1
		}
		pw[k+1] = pw[k] + cw[k]
		pl[k+1] = pl[k]
		r, _ := utf8.DecodeRuneInString(c)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			pl[k+1]++
			lets = append(lets, k)
		}
	}
	hw := 1
	if size >= 2 {
		hw = glyph.Width(size)
	}
	// fits reports whether cs[i:j] (plus a hyphen if hy) fits.
	fits := func(i, j int, hy bool) bool {
		cnt := j - i
		L := pl[j] - pl[i]
		var fi int
		if L == 0 {
			nn := cnt
			if hy {
				nn++
			}
			fi = (nn - 1) / 2
		} else {
			var r int
			switch {
			case L <= 1:
				r = 0
			case L <= 5:
				r = 1
			case L <= 9:
				r = 2
			case L <= 13:
				r = 3
			default:
				r = 4
			}
			fi = lets[pl[i]+r] - i
		}
		total := pw[j] - pw[i]
		if hy {
			total += hw
		}
		var left int
		if fi < cnt {
			left = pw[i+fi] - pw[i]
		} else {
			left = total // focus is the hyphen
			left -= hw
		}
		if size >= 2 {
			W := hw
			return fi*W <= w/2-W/2 && w/2-W/2+(cnt+b2i(hy)-fi)*W <= w
		}
		return left <= w/2 && w/2+(total-left) <= w
	}
	if fits(0, n, false) {
		return []string{word}
	}
	var out []string
	for i := 0; i < n; {
		j := i + 1
		for j < n && fits(i, j+1, j+1 < n) {
			j++
		}
		s := strings.Join(cs[i:j], "")
		if j < n {
			s += "-"
		}
		out = append(out, s)
		i = j
	}
	return out
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
