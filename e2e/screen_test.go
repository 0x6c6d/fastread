//go:build e2e

package e2e

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

// cell is one grapheme cluster of a captured pane row with the SGR state it was drawn in.
type cell struct {
	Text    string
	Col     int
	Focus   bool
	Reverse bool
	Colored bool
}

type sgrState struct {
	reverse bool
	red     bool
	colored bool
}

func (st *sgrState) apply(params string) {
	if params == "" {
		*st = sgrState{}
		return
	}
	p := strings.Split(params, ";")
	num := func(i int) int {
		if i >= len(p) {
			return -1
		}
		n, err := strconv.Atoi(p[i])
		if err != nil {
			return -1
		}
		return n
	}
	for i := 0; i < len(p); i++ {
		n := num(i)
		switch {
		case p[i] == "" || n == 0:
			*st = sgrState{}
		case n == 7:
			st.reverse = true
		case n == 27:
			st.reverse = false
		case n == 31 || n == 91:
			st.red, st.colored = true, true
		case (n >= 30 && n <= 37) || (n >= 90 && n <= 97):
			st.red, st.colored = false, true
		case n == 39:
			st.red, st.colored = false, false
		case n == 38:
			switch num(i + 1) {
			case 5:
				c := num(i + 2)
				st.red, st.colored = c == 9 || c == 196, true
				i += 2
			case 2:
				st.red = num(i+2) == 255 && num(i+3) == 0 && num(i+4) == 0
				st.colored = true
				i += 4
			}
		}
	}
}

// parseScreen splits a capture(true) string into rows of cells.
func parseScreen(s string) [][]cell {
	var rows [][]cell
	for _, line := range strings.Split(s, "\n") {
		var row []cell
		var st sgrState
		col := 0
		var text strings.Builder
		flush := func() {
			rest := text.String()
			text.Reset()
			state := uniseg.NewGraphemes(rest)
			for state.Next() {
				w := state.Width()
				if w < 1 {
					w = 1
				}
				row = append(row, cell{
					Text: state.Str(), Col: col,
					Focus:   st.red || st.reverse,
					Reverse: st.reverse,
					Colored: st.colored,
				})
				col += w
			}
		}
		for i := 0; i < len(line); {
			if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
				j := i + 2
				for j < len(line) && (line[j] == ';' || line[j] == ':' || (line[j] >= '0' && line[j] <= '9')) {
					j++
				}
				if j < len(line) && line[j] == 'm' {
					flush()
					st.apply(line[i+2 : j])
				}
				if j < len(line) {
					j++
				}
				i = j
				continue
			}
			text.WriteByte(line[i])
			i++
		}
		flush()
		rows = append(rows, row)
	}
	return rows
}

// focusCols returns the sorted distinct columns of non-space Focus cells.
func focusCols(rows [][]cell) []int {
	set := map[int]bool{}
	for _, r := range rows {
		for _, c := range r {
			if c.Focus && strings.TrimSpace(c.Text) != "" {
				set[c.Col] = true
			}
		}
	}
	out := []int{}
	for c := range set {
		out = append(out, c)
	}
	sort.Ints(out)
	return out
}

// tickCols returns (row, col) of every "│" cell.
func tickCols(rows [][]cell) [][2]int {
	var out [][2]int
	for ri, r := range rows {
		for _, c := range r {
			if c.Text == "│" {
				out = append(out, [2]int{ri, c.Col})
			}
		}
	}
	return out
}

func TestParseScreen(t *testing.T) {
	esc := "\x1b"
	for _, tc := range []struct{ name, seq string }{
		{"31", esc + "[31m"}, {"91", esc + "[91m"}, {"256-9", esc + "[38;5;9m"},
		{"256-196", esc + "[38;5;196m"}, {"truecolor", esc + "[38;2;255;0;0m"},
	} {
		rows := parseScreen("ab" + tc.seq + "X" + esc + "[0mcd\n")
		if got := focusCols(rows); !reflect.DeepEqual(got, []int{2}) {
			t.Errorf("%s: focusCols=%v", tc.name, got)
		}
		if !rows[0][2].Colored || rows[0][3].Colored || rows[0][3].Focus {
			t.Errorf("%s: flags wrong: %+v", tc.name, rows[0])
		}
	}

	rows := parseScreen("a" + esc + "[7mB" + esc + "[27mc" + esc + "[32mG" + esc + "[39md")
	r := rows[0]
	if !r[1].Reverse || !r[1].Focus || r[1].Colored || r[2].Focus || !r[3].Colored || r[3].Focus || r[4].Colored {
		t.Errorf("reverse/green flags wrong: %+v", r)
	}

	// Reset in the middle, empty-parameter reset.
	rows = parseScreen(esc + "[1;31mAB" + esc + "[mCD" + esc + "[31mE")
	if got := focusCols(rows); !reflect.DeepEqual(got, []int{0, 1, 4}) {
		t.Errorf("reset: focusCols=%v", got)
	}

	// Wide CJK cluster takes two columns; combining cluster one.
	rows = parseScreen("日" + esc + "[31m" + "é" + esc + "[0m│\nx")
	r = rows[0]
	if len(r) != 3 || r[0].Col != 0 || r[1].Col != 2 || r[1].Text != "é" || r[2].Col != 3 {
		t.Errorf("wide/combining columns wrong: %+v", r)
	}
	if got := tickCols(rows); !reflect.DeepEqual(got, [][2]int{{0, 3}}) {
		t.Errorf("tickCols=%v", got)
	}
	if len(rows) != 2 || len(rows[1]) != 1 {
		t.Errorf("rows=%+v", rows)
	}
}
