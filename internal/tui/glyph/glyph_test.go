package glyph

import (
	"reflect"
	"strings"
	"testing"
)

func TestGlyphTables(t *testing.T) {
	rw := []struct{ size, rows, width int }{
		{0, 0, 0}, {1, 1, 1}, {2, 3, 4}, {3, 5, 6}, {4, 7, 8}, {5, 9, 10}, {6, 0, 0},
	}
	for _, c := range rw {
		if Rows(c.size) != c.rows || Width(c.size) != c.width {
			t.Errorf("size %d: Rows/Width = %d/%d, want %d/%d", c.size, Rows(c.size), Width(c.size), c.rows, c.width)
		}
	}

	var covered []rune
	for r := rune(0x20); r <= 0x7E; r++ {
		covered = append(covered, r)
	}
	for r := rune(0xA0); r <= 0xFF; r++ {
		covered = append(covered, r)
	}
	for size := 2; size <= 5; size++ {
		for _, r := range covered {
			rows, ok := Block(r, size)
			if !ok || !Has(r) {
				t.Fatalf("Block(%q,%d) not ok", r, size)
			}
			if len(rows) != Rows(size) {
				t.Fatalf("Block(%q,%d): %d rows", r, size, len(rows))
			}
			ink := false
			for _, row := range rows {
				if n := len([]rune(row)); n != Width(size) {
					t.Fatalf("Block(%q,%d): row %q has %d runes", r, size, row, n)
				}
				if strings.Trim(row, " ▀▄█") != "" {
					t.Fatalf("Block(%q,%d): bad rune in %q", r, size, row)
				}
				if strings.Trim(row, " ") != "" {
					ink = true
				}
			}
			if r >= 0x21 && r <= 0x7E && !ink {
				t.Errorf("Block(%q,%d) has no ink", r, size)
			}
			if (r == ' ' || r == 0xA0) && ink {
				t.Errorf("Block(%q,%d) has ink", r, size)
			}
		}
	}

	for _, r := range []rune{0x1F, 0x7F, 0x80, 0x9F, 0x100, '€', '日', '—', -1} {
		if Has(r) {
			t.Errorf("Has(%q) true", r)
		}
		if _, ok := Block(r, 3); ok {
			t.Errorf("Block(%q,3) ok", r)
		}
	}
	for _, size := range []int{1, 6} {
		if _, ok := Block('A', size); ok {
			t.Errorf("Block('A',%d) ok", size)
		}
	}

	sp4, sp5 := strings.Repeat(" ", 8), strings.Repeat(" ", 10)
	lit := []struct {
		r    rune
		size int
		want []string
	}{
		{'H', 2, []string{"█ █ ", "█▀█ ", "▀ ▀ "}},
		{'H', 3, []string{"▄   ▄ ", "█   █ ", "█▀▀▀█ ", "█   █ ", "      "}},
		{'é', 3, []string{"  ▄▀  ", " ▄▄▄  ", "█▄▄▄█ ", "▀▄▄▄  ", "      "}},
		{'-', 4, []string{sp4, sp4, sp4, " ▄▄▄▄▄  ", sp4, sp4, sp4}},
		{'-', 5, []string{sp5, sp5, sp5, sp5, " ▄▄▄▄▄▄▄  ", sp5, sp5, sp5, sp5}},
	}
	for _, c := range lit {
		got, _ := Block(c.r, c.size)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Block(%q,%d) = %q, want %q", c.r, c.size, got, c.want)
		}
	}
}
