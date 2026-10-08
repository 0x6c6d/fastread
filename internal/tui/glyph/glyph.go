// Package glyph holds the embedded block-glyph font for TUI sizes 2–5.
package glyph

import "sync"

var (
	once   sync.Once
	blocks [6][][]string // [size][slot] -> rows
)

var pixRows = [6]int{0, 0, 6, 10, 14, 18}

// Rows returns the number of terminal rows of a glyph at size 1..5, else 0.
func Rows(size int) int {
	if size < 1 || size > 5 {
		return 0
	}
	return 2*size - 1
}

// Width returns the number of terminal columns of a glyph at size 1..5, else 0.
func Width(size int) int {
	if size < 1 || size > 5 {
		return 0
	}
	return [...]int{0, 1, 4, 6, 8, 10}[size]
}

// Has reports whether the font covers r.
func Has(r rune) bool { return (r >= 0x20 && r <= 0x7E) || (r >= 0xA0 && r <= 0xFF) }

func slot(r rune) int {
	if r <= 0x7E {
		return int(r - 0x20)
	}
	return int(r-0xA0) + 95
}

func build() {
	tables := [6][]uint16{2: font2, 3: font3, 4: font4, 5: font5}
	for size := 2; size <= 5; size++ {
		w, ph := Width(size), pixRows[size]
		n := len(tables[size]) / ph
		blocks[size] = make([][]string, n)
		for s := 0; s < n; s++ {
			px := tables[size][s*ph : (s+1)*ph]
			rows := make([]string, ph/2)
			for y := range rows {
				out := make([]rune, w)
				for x := 0; x < w; x++ {
					top := px[2*y]&(1<<(w-1-x)) != 0
					bot := px[2*y+1]&(1<<(w-1-x)) != 0
					switch {
					case top && bot:
						out[x] = '█'
					case top:
						out[x] = '▀'
					case bot:
						out[x] = '▄'
					default:
						out[x] = ' '
					}
				}
				rows[y] = string(out)
			}
			blocks[size][s] = rows
		}
	}
}

// Block returns the glyph of r at size 2..5 as Rows(size) strings of exactly Width(size)
// runes; cell row y shows pixel rows 2y (top) and 2y+1 (bottom): both ink '█', top only
// '▀', bottom only '▄', neither ' '. ok is false when !Has(r) or size is not 2..5.
// The result is shared and must not be modified by callers.
func Block(r rune, size int) (rows []string, ok bool) {
	if !Has(r) || size < 2 || size > 5 {
		return nil, false
	}
	once.Do(build)
	return blocks[size][slot(r)], true
}
