package tui

import (
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/orp"
	"github.com/rivo/uniseg"
)

func TestWideAndCombining(t *testing.T) {
	words := []string{
		"日本語の本", "한국어", "👍👍👍", "ok👍", "été", "café",
		"́abc", "a​b", "🇩🇪🇫🇷", "👩‍👩‍👧x", "́", "日本", "Ωmega",
	}
	const h = 24
	for _, word := range words {
		for _, w := range []int{80, 121} {
			for size := 1; size <= 5; size++ {
				f := Render(Model{Word: word, Size: size}, w, h)
				if f.Size != 1 {
					t.Fatalf("%q size %d w %d: Size = %d, want 1", word, size, w, f.Size)
				}
				cs := orp.Clusters(word)
				fc := cs[orp.Index(word)]
				fw := uniseg.StringWidth(fc)
				if fw == 0 {
					fc = "◌" + fc
				}
				row := f.Cells[h/2]
				nfocus := 0
				for y := range f.Cells {
					for x, c := range f.Cells[y] {
						if c.Style == StyleFocus {
							nfocus++
							if x != w/2 || y != h/2 || c.Text != fc {
								t.Errorf("%q w %d: focus at (%d,%d) %q, want (%d,%d) %q", word, w, x, y, c.Text, w/2, h/2, fc)
							}
						}
					}
				}
				if nfocus != 1 {
					t.Errorf("%q w %d: %d focus cells, want 1", word, w, nfocus)
				}
				if fw == 2 {
					c := row[w/2+1]
					if !c.Cont || c.Style != StylePlain || c.Text != "" {
						t.Errorf("%q w %d: bad Cont cell %+v", word, w, c)
					}
				}
				var sb strings.Builder
				occ, first, last := 0, -1, -1
				for x, c := range row {
					if c.Text == "" && !c.Cont {
						continue
					}
					occ++
					if first < 0 {
						first = x
					}
					last = x
					if !c.Cont {
						sb.WriteString(strings.TrimPrefix(c.Text, "◌"))
					}
				}
				want := 0
				for _, c := range cs {
					want += max(1, uniseg.StringWidth(c))
				}
				if sb.String() != word {
					t.Errorf("%q w %d: row reads %q", word, w, sb.String())
				}
				if occ != want || last-first+1 != occ {
					t.Errorf("%q w %d: occupied %d span %d, want %d contiguous", word, w, occ, last-first+1, want)
				}
			}
		}
	}
}

func TestWideAndCombiningSpots(t *testing.T) {
	row := func(word string) []Cell { return Render(Model{Word: word, Size: 1}, 80, 24).Cells[12] }
	r := row("ok👍")
	if r[39].Text != "o" || r[40].Text != "k" || r[41].Text != "👍" || !r[42].Cont {
		t.Errorf("ok👍 row wrong: %+v", r[38:44])
	}
	fam := "👩‍👩‍👧"
	r = row(fam + "x")
	if r[38].Text != fam || !r[39].Cont {
		t.Errorf("family row wrong: %+v", r[37:41])
	}
	r = row("́abc")
	if r[38].Text != "◌́" {
		t.Errorf("combining row wrong: %+v", r[37:41])
	}
}

func TestWideClip(t *testing.T) {
	for _, word := range []string{"a" + strings.Repeat("日", 14), strings.Repeat("日", 13) + "x"} {
		for _, w := range []int{20, 21} {
			f := Render(Model{Word: word, Size: 1}, w, 24)
			row := f.Cells[12]
			if row[0].Cont {
				t.Errorf("%q w %d: Cont at column 0", word, w)
			}
			for x, c := range row {
				if c.Cont && (x == 0 || row[x-1].Text == "" || uniseg.StringWidth(row[x-1].Text) != 2) {
					t.Errorf("%q w %d: orphan Cont at %d", word, w, x)
				}
			}
			if uniseg.StringWidth(row[w-1].Text) == 2 {
				t.Errorf("%q w %d: wide cluster in last column", word, w)
			}
			if row[w/2].Style != StyleFocus {
				t.Errorf("%q w %d: no focus at %d", word, w, w/2)
			}
		}
	}
}
