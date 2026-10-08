package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func gSerialise(name string, f Frame) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s w=%d h=%d size=%d parts=%d\n", name, f.W, f.H, f.Size, f.Parts)
	for _, row := range f.Cells {
		for _, c := range row {
			switch {
			case c.Cont:
			case c.Text == "":
				b.WriteString(" ")
			default:
				b.WriteString(c.Text)
			}
		}
		b.WriteString("|\n")
	}
	b.WriteString("--\n")
	for _, row := range f.Cells {
		for _, c := range row {
			switch c.Style {
			case StyleFocus:
				b.WriteByte('F')
			case StyleTick:
				b.WriteByte('T')
			default:
				b.WriteByte('.')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

type goldenCase struct {
	name string
	m    Model
	w, h int
	word bool // one of the 20 word cases
}

func goldenCases() []goldenCase {
	base := Model{ShowProgress: true, Index: 4, Total: 120, WPM: 300, EffectiveWPM: 287}
	with := func(word string, size int, f func(*Model)) Model {
		m := base
		m.Word, m.Size = word, size
		if f != nil {
			f(&m)
		}
		return m
	}
	var cs []goldenCase
	words := []struct{ stem, word string }{{"a", "a"}, {"hello", "Hello"}, {"wonderful", "wonderful"}, {"naive", "na\u00efve"}}
	for _, w := range words {
		for n := 1; n <= 5; n++ {
			cs = append(cs, goldenCase{fmt.Sprintf("%s_s%d", w.stem, n), with(w.word, n, nil), 80, 24, true})
		}
	}
	cs = append(cs,
		goldenCase{"hello_s3_paused", with("Hello", 3, func(m *Model) { m.Paused = true }), 80, 24, false},
		goldenCase{"hello_s2_progress_off", with("Hello", 2, func(m *Model) { m.ShowProgress = false }), 80, 24, false},
		goldenCase{"hello_s2_help_on", with("Hello", 2, func(m *Model) { m.ShowHelp = true }), 80, 24, false},
		goldenCase{"naive_s1_help_on_progress_off", with("na\u00efve", 1, func(m *Model) { m.ShowHelp, m.ShowProgress = true, false }), 80, 24, false},
		goldenCase{"wonderful_s5_part1", with("wonderful", 5, func(m *Model) { m.Part = 1 }), 80, 24, false},
		goldenCase{"nihon_s3_fallback", with("\u65e5\u672c", 3, nil), 80, 24, false},
		goldenCase{"hello_s5_40x8", with("Hello", 5, nil), 40, 8, false},
		goldenCase{"toosmall_19x4", with("Hello", 3, nil), 19, 4, false},
	)
	return cs
}

func gFocusRows(f Frame) []int {
	var rows []int
	for y, row := range f.Cells {
		for _, c := range row {
			if c.Style == StyleFocus {
				rows = append(rows, y)
				break
			}
		}
	}
	return rows
}

func gIsTick(f Frame, y, x int) bool {
	return y >= 0 && y < f.H && x < len(f.Cells[y]) && f.Cells[y][x].Style == StyleTick
}

func TestGolden(t *testing.T) {
	dir := filepath.Join("testdata", "golden")
	frames := map[string]Frame{}
	for _, c := range goldenCases() {
		c := c
		f := Render(c.m, c.w, c.h)
		frames[c.name] = f
		t.Run(c.name, func(t *testing.T) {
			got := gSerialise(c.name, f)
			path := filepath.Join(dir, c.name+".golden")
			if *update {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("golden missing: %v (run with -update)", err)
			}
			if string(want) != got {
				wl, gl := strings.Split(string(want), "\n"), strings.Split(got, "\n")
				for i := 0; i < len(wl) || i < len(gl); i++ {
					var a, b string
					if i < len(wl) {
						a = wl[i]
					}
					if i < len(gl) {
						b = gl[i]
					}
					if a != b {
						t.Fatalf("%s differs at line %d:\nwant %q\ngot  %q", c.name, i+1, a, b)
					}
				}
			}

			// Assertions computed from the frame.
			if f.Size >= 1 {
				rows := gFocusRows(f)
				if len(rows) == 0 {
					t.Fatalf("no focus rows")
				}
				first, last := rows[0], rows[len(rows)-1]
				if !gIsTick(f, first-1, f.W/2) || !gIsTick(f, last+1, f.W/2) {
					t.Errorf("missing tick at focus column around focus rows %d..%d", first, last)
				}
			}
			if c.word {
				var n int
				fmt.Sscanf(c.name[strings.LastIndex(c.name, "_s")+2:], "%d", &n)
				if got, want := len(gFocusRows(f)), 2*n-1; got != want {
					t.Errorf("focus rows = %d, want %d", got, want)
				}
			}
		})
	}
	same := func(a, b string) {
		fa, fb := frames[a], frames[b]
		ra, rb := gFocusRows(fa), gFocusRows(fb)
		if fmt.Sprint(ra) != fmt.Sprint(rb) {
			t.Errorf("%s vs %s: word rows %v vs %v", a, b, ra, rb)
			return
		}
		for _, y := range ra {
			for x := range fa.Cells[y] {
				if (fa.Cells[y][x].Style == StyleFocus) != (fb.Cells[y][x].Style == StyleFocus) {
					t.Errorf("%s vs %s: focus column differs at %d,%d", a, b, y, x)
					return
				}
			}
		}
	}
	same("hello_s2_progress_off", "hello_s2")
	same("hello_s2_help_on", "hello_s2")
	same("naive_s1_help_on_progress_off", "naive_s1")

	hdr := map[string][3]int{
		"wonderful_s3": {3, 1, 0}, "wonderful_s4": {4, 2, 0}, "wonderful_s5": {5, 2, 0},
		"nihon_s3_fallback": {1, 0, 0}, "hello_s5_40x8": {2, 0, 0}, "toosmall_19x4": {0, 0, 0},
	}
	for n, w := range hdr {
		f := frames[n]
		if n == "wonderful_s3" || n == "wonderful_s4" || n == "wonderful_s5" {
			if f.Parts != w[1] {
				t.Errorf("%s parts=%d want %d", n, f.Parts, w[1])
			}
			continue
		}
		if f.Size != map[string]int{"nihon_s3_fallback": 1, "hello_s5_40x8": 2, "toosmall_19x4": 0}[n] {
			t.Errorf("%s size=%d", n, f.Size)
		}
	}
	if f := frames["toosmall_19x4"]; f.Parts != 0 {
		t.Errorf("toosmall parts=%d", f.Parts)
	}
}
