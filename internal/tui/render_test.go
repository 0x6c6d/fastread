package tui

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/orp"
)

func TestRenderFocusLevel1(t *testing.T) {
	words := []string{"a", "Hello", "wonderful", `"extraordinary"`, "naïve", "--", "(wonderful)", "é日本"}
	for _, word := range words {
		for _, w := range []int{80, 121, 20} {
			for _, h := range []int{24, 5} {
				f := Render(Model{Word: word, Size: 1}, w, h)
				n := 0
				var sb strings.Builder
				for y, row := range f.Cells {
					for x, c := range row {
						if c.Style == StyleFocus {
							n++
							want := orp.Clusters(word)[orp.Index(word)]
							if x != w/2 || y != h/2 || c.Text != want {
								t.Errorf("%q %dx%d: focus at (%d,%d) %q", word, w, h, x, y, c.Text)
							}
						}
						if y == h/2 && !c.Cont {
							sb.WriteString(c.Text)
						}
					}
				}
				if n != 1 {
					t.Errorf("%q %dx%d: %d focus cells", word, w, h, n)
				}
				if w != 20 && sb.String() != word {
					t.Errorf("%q %dx%d: row reads %q", word, w, h, sb.String())
				}
			}
		}
	}
}

func TestRenderClip(t *testing.T) {
	cases := []struct {
		word string
		w, h int
	}{{strings.Repeat("a", 60), 20, 5}, {"Hello", 0, 0}, {"Hello", 1, 1}, {"Hello", 3, 2}, {"é日本", 1, 1}}
	for _, c := range cases {
		f := Render(Model{Word: c.word}, c.w, c.h)
		if len(f.Cells) != c.h {
			t.Errorf("%v: rows %d", c, len(f.Cells))
		}
		for _, r := range f.Cells {
			if len(r) != c.w {
				t.Errorf("%v: cols %d", c, len(r))
			}
		}
	}
}

func TestEncodeColors(t *testing.T) {
	focus := map[ColorMode]string{
		ColorTrue: "\x1b[38;2;255;0;0m", Color256: "\x1b[38;5;196m",
		Color16: "\x1b[91m", ColorNone: "\x1b[1;7m",
	}
	colour := regexp.MustCompile(`\x1b\[(3[0-9]|9[0-7])(;[0-9]+)*m`)
	bg := regexp.MustCompile(`\x1b\[4[0-9]|48;`)
	f := Render(Model{Word: "Hello"}, 20, 5)
	for mode, s := range focus {
		out := Encode(f, mode)
		if !bytes.Contains(out, []byte(s+"e")) {
			t.Errorf("mode %d: missing %q before e in %q", mode, s, out)
		}
		if mode == ColorNone && colour.Match(out) {
			t.Errorf("ColorNone has colour: %q", out)
		}
		if bg.Match(out) {
			t.Errorf("mode %d: background SGR in %q", mode, out)
		}
	}
}

func TestDetectColorMode(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want ColorMode
	}{
		{map[string]string{"NO_COLOR": "1", "COLORTERM": "truecolor"}, ColorNone},
		{map[string]string{"COLORTERM": "24bit"}, ColorTrue},
		{map[string]string{"TERM": "xterm-256color"}, Color256},
		{map[string]string{"TERM": "xterm"}, Color16},
		{map[string]string{"NO_COLOR": "", "TERM": "screen-256color"}, Color256},
	}
	for _, c := range cases {
		if got := DetectColorMode(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("%v: got %d want %d", c.env, got, c.want)
		}
	}
}
