package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/0x6c6d/fastread/internal/tokenize"
)

var csiRE = regexp.MustCompile(`\x1b\[[0-9;]*[Hm]`)

func stripCSI(b []byte) string { return string(csiRE.ReplaceAll(b, nil)) }

var allModes = []ColorMode{ColorNone, Color16, Color256, ColorTrue}

// leak returns a description of the first leak in out, or "".
func leak(out []byte) string {
	if !utf8.Valid(out) {
		return "invalid UTF-8"
	}
	for _, r := range stripCSI(out) {
		if r <= 0x1F || (r >= 0x7F && r <= 0x9F) || (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) {
			return fmt.Sprintf("control rune %U", r)
		}
	}
	return ""
}

var hostile = []string{
	"\x1b[2J", "\x1b]0;pwned\x07", "\u009b31m", "\xff\xfe", "a\x00b", "‮evil",
	"⁦x⁩", "\x7f", "\r\n", "é", "👍",
}

const hostileWord = "a\x1b[31mb\x9bc‮d\x07e\x00f\xffg\x1b]0;x\x07"

func rowFrame(texts ...string) Frame {
	row := make([]Cell, len(texts))
	for i, s := range texts {
		row[i] = Cell{Text: s}
	}
	return Frame{W: len(texts), H: 1, Cells: [][]Cell{row}}
}

func TestEncodeStripsControls(t *testing.T) {
	t.Run("a", func(t *testing.T) {
		f := rowFrame("a\x1bb", "\x07", "c", "\x9b")
		for _, m := range allModes {
			out := Encode(f, m)
			if got := stripCSI(out); got != "ab c�" {
				t.Errorf("mode %d: got %q want %q", m, got, "ab c�")
			}
			if strings.IndexByte(string(out), 0x9b) >= 0 {
				t.Errorf("mode %d: raw 0x9B in %q", m, out)
			}
		}
	})
	t.Run("b", func(t *testing.T) {
		texts := append(append([]string{}, hostile...), "")
		f := rowFrame(texts...)
		f.Cells[0][len(texts)-1] = Cell{Cont: true} // the cell after 👍
		for _, m := range allModes {
			out := Encode(f, m)
			if why := leak(out); why != "" {
				t.Errorf("mode %d: %s in %q", m, why, out)
			}
			vis := stripCSI(out)
			for _, want := range []string{"[2J", "]0;pwned", "evil", "é", "👍"} {
				if !strings.Contains(vis, want) {
					t.Errorf("mode %d: %q not visible in %q", m, want, vis)
				}
			}
		}
	})
	t.Run("c", func(t *testing.T) {
		for _, size := range []int{1, 3} {
			for _, prog := range []bool{false, true} {
				for _, help := range []bool{false, true} {
					f := Render(Model{Word: hostileWord, Size: size, ShowProgress: prog, ShowHelp: help, Total: 1}, 80, 24)
					for _, m := range allModes {
						if why := leak(Encode(f, m)); why != "" {
							t.Errorf("size %d prog %v help %v mode %d: %s", size, prog, help, m, why)
						}
					}
				}
			}
		}
	})
	t.Run("d", func(t *testing.T) {
		for r := rune(0); r <= utf8.MaxRune; r++ {
			if r >= 0xD800 && r <= 0xDFFF {
				continue
			}
			s := string(r)
			if got, want := cleanCell(s), tokenize.Sanitize(s); got != want {
				t.Fatalf("%U: cleanCell %q, Sanitize %q", r, got, want)
			}
		}
		for _, s := range append(append([]string{}, hostile...), hostileWord, "a\x1bb", "\x07", "\x9b", "", "plain") {
			if got, want := cleanCell(s), tokenize.Sanitize(s); got != want {
				t.Errorf("%q: cleanCell %q, Sanitize %q", s, got, want)
			}
		}
	})
	t.Run("e", func(t *testing.T) {
		long := []Cell{{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"}, {Text: "e"}, {Text: "f"}, {Text: "g"}}
		short := []Cell{{Text: "x"}}
		cases := []struct {
			name string
			f    Frame
		}{
			{"nil cells", Frame{W: 5, H: 3}},
			{"short and long rows", Frame{W: 5, H: 3, Cells: [][]Cell{short, long, nil}}},
			{"more rows than H", Frame{W: 5, H: 1, Cells: [][]Cell{long, long}}},
			{"W -1", Frame{W: -1, H: 2, Cells: [][]Cell{long, long}}},
			{"H -1", Frame{W: 5, H: -1, Cells: [][]Cell{long}}},
		}
		for _, c := range cases {
			for _, m := range allModes {
				out := Encode(c.f, m)
				w := max(c.f.W, 0)
				for _, line := range splitRows(out) {
					if n := utf8.RuneCountInString(line); n > w {
						t.Errorf("%s mode %d: row %q has %d runes > W %d", c.name, m, line, n, w)
					}
				}
				if (c.f.W <= 0 || c.f.H <= 0) && len(out) != 0 {
					t.Errorf("%s: want empty output, got %q", c.name, out)
				}
				if n := len(splitRows(out)); n > max(c.f.H, 0) {
					t.Errorf("%s: %d rows > H %d", c.name, n, c.f.H)
				}
			}
		}
	})
}

var cupRE = regexp.MustCompile(`\x1b\[[0-9]+;1H`)

// splitRows splits encoded output at each cursor move and strips the SGRs of every row.
func splitRows(out []byte) []string {
	var rows []string
	idx := cupRE.FindAllIndex(out, -1)
	for i, loc := range idx {
		end := len(out)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		rows = append(rows, stripCSI(out[loc[1]:end]))
	}
	return rows
}

func FuzzRenderEncode(f *testing.F) {
	for _, s := range append(append([]string{}, hostile...), hostileWord, "Hello", "") {
		f.Add(s, uint8(1), uint8(80), uint8(24), uint8(3))
		f.Add(s, uint8(3), uint8(40), uint8(12), uint8(0))
	}
	f.Fuzz(func(t *testing.T, word string, size, w, h, mode uint8) {
		W, H := int(w)%161, int(h)%61
		fr := Render(Model{Word: word, Size: int(size) % 7, ShowProgress: true, ShowHelp: true, Total: 1}, W, H)
		if fr.W != W || fr.H != H {
			t.Fatalf("frame %dx%d, want %dx%d", fr.W, fr.H, W, H)
		}
		if why := leak(Encode(fr, ColorMode(mode%4))); why != "" {
			t.Fatalf("%q: %s", word, why)
		}
	})
}

var (
	focusSGR = map[ColorMode]string{
		ColorTrue: "\x1b[38;2;255;0;0m", Color256: "\x1b[38;5;196m",
		Color16: "\x1b[91m", ColorNone: "\x1b[1;7m",
	}
	plainSGR = map[ColorMode]string{
		ColorTrue: "\x1b[38;2;255;255;255m", Color256: "\x1b[38;5;231m", Color16: "\x1b[97m",
	}
	bgRE     = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*(?:4[0-9]|10[0-7])(?:;[0-9]+)*m`)
	fgRE     = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*(?:3[0-9]|9[0-7])(?:;[0-9]+)*m`)
	anySGRRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// focusRuns checks that sgr occurs n times in out, each followed by want runes from the
// box alphabet (or exactly the string exact when non-empty), then a reset.
func focusRuns(t *testing.T, label, out, sgr string, n, runes int, exact string) {
	t.Helper()
	if c := strings.Count(out, sgr); c != n {
		t.Errorf("%s: %q occurs %d times, want %d", label, sgr, c, n)
		return
	}
	rest := out
	for i := 0; i < n; i++ {
		rest = rest[strings.Index(rest, sgr)+len(sgr):]
		end := strings.Index(rest, "\x1b")
		if end < 0 || !strings.HasPrefix(rest[end:], "\x1b[0m") {
			t.Errorf("%s: occurrence %d not followed by text then reset", label, i)
			continue
		}
		seg := rest[:end]
		if exact != "" {
			if seg != exact {
				t.Errorf("%s: occurrence %d followed by %q want %q", label, i, seg, exact)
			}
			continue
		}
		if utf8.RuneCountInString(seg) != runes || strings.Trim(seg, " ▀▄█") != "" {
			t.Errorf("%s: occurrence %d followed by %q, want %d box runes", label, i, seg, runes)
		}
	}
}

func TestColorModes(t *testing.T) {
	env := []struct {
		env  map[string]string
		want ColorMode
	}{
		{map[string]string{"COLORTERM": "truecolor"}, ColorTrue},
		{map[string]string{"COLORTERM": "24bit"}, ColorTrue},
		{map[string]string{"TERM": "xterm-256color"}, Color256},
		{map[string]string{"TERM": "screen-256color"}, Color256},
		{map[string]string{"TERM": "xterm"}, Color16},
		{map[string]string{}, Color16},
	}
	for _, c := range env {
		if got := DetectColorMode(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("%v: got %d want %d", c.env, got, c.want)
		}
	}
	for _, m := range []ColorMode{Color16, Color256, ColorTrue} {
		out := string(Encode(Render(Model{Word: "Hello", Size: 1, ShowProgress: true, ShowHelp: true, Total: 3}, 80, 24), m))
		focusRuns(t, "size 1", out, focusSGR[m], 1, 0, "e")
		for o, s := range focusSGR {
			if o != m && strings.Contains(out, s) {
				t.Errorf("mode %d: other mode's focus SGR %q present", m, s)
			}
		}
		if !strings.Contains(out, plainSGR[m]) {
			t.Errorf("mode %d: plain SGR %q missing", m, plainSGR[m])
		}
		if strings.Contains(out, "48;") || bgRE.MatchString(out) {
			t.Errorf("mode %d: background SGR in %q", m, out)
		}
		if n := strings.Count(out, "│"); n != 2 {
			t.Errorf("mode %d: %d ticks, want 2", m, n)
		}
		if strings.Count(out, "\x1b[0m│") != strings.Count(out, "│") {
			t.Errorf("mode %d: a tick is not immediately preceded by a reset: %q", m, out)
		}
		out3 := string(Encode(Render(Model{Word: "Hello", Size: 3, ShowProgress: true, ShowHelp: true, Total: 3}, 80, 24), m))
		focusRuns(t, "size 3", out3, focusSGR[m], 5, 6, "")
	}
}

func TestNoColor(t *testing.T) {
	env := []struct {
		env  map[string]string
		want ColorMode
	}{
		{map[string]string{"NO_COLOR": "1", "COLORTERM": "truecolor", "TERM": "xterm-256color"}, ColorNone},
		{map[string]string{"NO_COLOR": "yes", "COLORTERM": "truecolor", "TERM": "xterm-256color"}, ColorNone},
		{map[string]string{"NO_COLOR": "", "TERM": "xterm-256color"}, Color256},
	}
	for _, c := range env {
		if got := DetectColorMode(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("%v: got %d want %d", c.env, got, c.want)
		}
	}
	for _, size := range []int{1, 3} {
		for _, prog := range []bool{false, true} {
			for _, help := range []bool{false, true} {
				out := string(Encode(Render(Model{Word: "Hello", Size: size, ShowProgress: prog, ShowHelp: help, Total: 3}, 80, 24), ColorNone))
				if strings.Contains(out, "\x1b[38") || fgRE.MatchString(out) {
					t.Errorf("size %d: colour SGR in %q", size, out)
				}
				for _, s := range anySGRRE.FindAllString(out, -1) {
					if s != "\x1b[0m" && s != "\x1b[1;7m" {
						t.Errorf("size %d: unexpected SGR %q", size, s)
					}
				}
				if size == 1 {
					focusRuns(t, "none size 1", out, "\x1b[1;7m", 1, 0, "e")
				} else {
					focusRuns(t, "none size 3", out, "\x1b[1;7m", 5, 6, "")
				}
			}
		}
	}
}
