package tui

import (
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/tui/glyph"
)

func TestTicks(t *testing.T) {
	dims := [][2]int{{80, 24}, {121, 30}, {40, 8}, {20, 5}}
	for _, word := range []string{"a", "Hello", "naïve", "日本"} {
		for size := 1; size <= 5; size++ {
			for _, d := range dims {
				w, h := d[0], d[1]
				for _, prog := range []bool{false, true} {
					m := Model{Word: word, Size: size, ShowProgress: prog}
					play := Render(m, w, h)
					m.Paused = true
					pause := Render(m, w, h)
					var ticks, focus []int
					for y, row := range play.Cells {
						for x, c := range row {
							if c.Style == StyleTick {
								if x != w/2 || c.Text != "│" {
									t.Fatalf("%q s%d %dx%d: bad tick at %d,%d %q", word, size, w, h, x, y, c.Text)
								}
								ticks = append(ticks, y)
							}
							if c.Style == StyleFocus {
								if len(focus) == 0 || focus[len(focus)-1] != y {
									focus = append(focus, y)
								}
							}
							if pause.Cells[y][x] != c {
								t.Fatalf("%q s%d %dx%d: paused differs at %d,%d", word, size, w, h, x, y)
							}
						}
					}
					R := glyph.Rows(play.Size)
					if len(ticks) != 2 || len(focus) != R {
						t.Fatalf("%q s%d %dx%d: ticks %v focus rows %v R=%d", word, size, w, h, ticks, focus, R)
					}
					y0 := focus[0]
					if ticks[0] != y0-1 || ticks[1] != y0+R || ticks[0] < 1 || ticks[1] > h-2 {
						t.Fatalf("%q s%d %dx%d: ticks %v y0=%d R=%d", word, size, w, h, ticks, y0, R)
					}
				}
			}
		}
	}
}

func tickRows(f Frame) []int {
	var r []int
	for y, row := range f.Cells {
		for _, c := range row {
			if c.Style == StyleTick {
				r = append(r, y)
			}
		}
	}
	return r
}

func TestTicksSpot(t *testing.T) {
	tests := []struct {
		w, h, size int
		want       []int
	}{
		{80, 24, 1, []int{11, 13}},
		{80, 24, 3, []int{9, 15}},
		{80, 24, 5, []int{7, 17}},
		{40, 8, 2, []int{2, 6}},
		{20, 5, 1, []int{1, 3}},
	}
	for _, tc := range tests {
		f := Render(Model{Word: "Hello", Size: tc.size}, tc.w, tc.h)
		got := tickRows(f)
		if f.Size != tc.size || len(got) != 2 || got[0] != tc.want[0] || got[1] != tc.want[1] {
			t.Errorf("%dx%d size %d: Size %d ticks %v want %v", tc.w, tc.h, tc.size, f.Size, got, tc.want)
		}
	}
}

func TestTooSmall(t *testing.T) {
	tests := []struct{ w, h int }{{19, 4}, {19, 24}, {80, 4}, {18, 5}, {5, 2}, {1, 1}, {0, 0}, {-3, 7}}
	for _, tc := range tests {
		f := Render(Model{Word: "Hello", Size: 3}, tc.w, tc.h)
		w, h := max(tc.w, 0), max(tc.h, 0)
		if f.Size != 0 || len(f.Cells) != h {
			t.Fatalf("%dx%d: Size %d rows %d", tc.w, tc.h, f.Size, len(f.Cells))
		}
		for y, row := range f.Cells {
			if len(row) != w {
				t.Fatalf("%dx%d: row %d len %d", tc.w, tc.h, y, len(row))
			}
			var sb strings.Builder
			for _, c := range row {
				if c.Style == StyleFocus || c.Style == StyleTick {
					t.Fatalf("%dx%d: styled cell", tc.w, tc.h)
				}
				if c.Text == "" {
					sb.WriteByte(' ')
				} else {
					sb.WriteString(c.Text)
				}
			}
			got := strings.TrimSpace(sb.String())
			want := ""
			if y == h/2 {
				x0 := max(0, (w-18)/2)
				want = strings.TrimSpace(TooSmallText[:min(18, w-x0)])
				if idx := strings.Index(sb.String(), got); got != "" && idx != x0 {
					t.Fatalf("%dx%d: text starts at %d want %d", tc.w, tc.h, idx, x0)
				}
			}
			if got != want {
				t.Fatalf("%dx%d: row %d = %q want %q", tc.w, tc.h, y, got, want)
			}
		}
	}
	f := Render(Model{Word: "Hello", Size: 3}, 20, 5)
	if f.Size != 1 || f.Cells[2][10].Style != StyleFocus {
		t.Fatalf("20x5: Size %d", f.Size)
	}
}
