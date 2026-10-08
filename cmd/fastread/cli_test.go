package main

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/tui"
)

// uiReached makes a valid command line stop at the UI with exit 1 and no terminal.
func uiReached() (tui.Terminal, error) { return nil, errors.New("ui reached") }

// runCLI runs the program with stdin non-TTY and the openTerminal seam set to open.
func runCLI(t *testing.T, args []string, stdin io.Reader, open func() (tui.Terminal, error)) (code int, stdout, stderr string, opened int) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	n, _ := seams(t, false, open)
	var out, errb bytes.Buffer
	code = run(args, stdin, &out, &errb, noenv)
	return code, out.String(), errb.String(), *n
}

// TestFlags is AC9: the R11 flag contract through run.
func TestFlags(t *testing.T) {
	bad := []struct {
		name string
		args []string
	}{
		{"--wpm 49", []string{"--wpm", "49", "a"}},
		{"--wpm 1501", []string{"--wpm", "1501", "a"}},
		{"--wpm x", []string{"--wpm", "x", "a"}},
		{"--wpm=", []string{"--wpm=", "a"}},
		{"--wpm=49", []string{"--wpm=49", "a"}},
		{"-wpm 49", []string{"-wpm", "49", "a"}},
		{"--size 0", []string{"--size", "0", "a"}},
		{"--size 6", []string{"--size", "6", "a"}},
		{"--size 2.5", []string{"--size", "2.5", "a"}},
		{"--ui foo", []string{"--ui", "foo", "a"}},
		{"--ui TUI", []string{"--ui", "TUI", "a"}},
		{"--start -1", []string{"--start", "-1", "a"}},
		{"--start 9999", []string{"--start", "9999", "a"}},
		{"--start 2 of 2 words", []string{"--start", "2", "a b"}},
		{"--bogus", []string{"--bogus", "a"}},
		{"-x", []string{"-x", "a"}},
		{"--wpm missing value", []string{"--wpm"}},
		{"two positionals", []string{"a", "b"}},
		{"flag after positional", []string{"hello", "--wpm", "100"}},
		{"--no-resume=maybe", []string{"--no-resume=maybe", "a"}},
	}
	for _, tc := range bad {
		t.Run("usage/"+tc.name, func(t *testing.T) {
			code, out, errs, opened := runCLI(t, tc.args, strings.NewReader(""), uiReached)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (stderr %q)", code, errs)
			}
			first, _, _ := strings.Cut(errs, "\n")
			if !strings.HasPrefix(first, "fastread: ") {
				t.Errorf("stderr line 1 = %q, want prefix %q", first, "fastread: ")
			}
			for _, s := range []string{"Usage:", "--wpm"} {
				if !strings.Contains(errs, s) {
					t.Errorf("stderr lacks %q: %q", s, errs)
				}
			}
			if strings.Contains(errs, "ui reached") || opened != 0 {
				t.Errorf("UI reached (opened %d): %q", opened, errs)
			}
			if out != "" {
				t.Errorf("stdout = %q, want empty", out)
			}
		})
	}

	good := []struct {
		name string
		args []string
	}{
		{"--wpm 50", []string{"--wpm", "50", "a"}},
		{"--wpm 1500", []string{"--wpm", "1500", "a"}},
		{"--wpm=300", []string{"--wpm=300", "a"}},
		{"--size 1", []string{"--size", "1", "a"}},
		{"--size 5", []string{"--size", "5", "a"}},
		{"--ui tui", []string{"--ui", "tui", "a"}},
		{"--start 0", []string{"--start", "0", "a"}},
		{"--start 1 of 2 words", []string{"--start", "1", "a b"}},
		{"--no-resume --no-progress", []string{"--no-resume", "--no-progress", "a"}},
		{"-- ends flags", []string{"--", "--wpm"}},
	}
	for _, tc := range good {
		t.Run("ui/"+tc.name, func(t *testing.T) {
			code, out, errs, opened := runCLI(t, tc.args, strings.NewReader(""), uiReached)
			if code != 1 {
				t.Errorf("exit = %d, want 1 (stderr %q)", code, errs)
			}
			if lines(errs) != 1 || !strings.Contains(errs, "ui reached") {
				t.Errorf("stderr = %q, want one line with %q", errs, "ui reached")
			}
			if opened != 1 {
				t.Errorf("openTerminal called %d times, want 1", opened)
			}
			if out != "" {
				t.Errorf("stdout = %q, want empty", out)
			}
		})
	}
}

// failReader fails the test if anything reads it.
type failReader struct{ t *testing.T }

func (r failReader) Read([]byte) (int, error) {
	r.t.Error("stdin was read")
	return 0, io.EOF
}

func TestHelpVersion(t *testing.T) {
	noUI := func(t *testing.T) func() (tui.Terminal, error) {
		return func() (tui.Terminal, error) {
			t.Error("openTerminal called")
			return nil, errors.New("ui reached")
		}
	}
	want := []string{"Usage:", "--wpm", "--size", "--ui", "--start", "--no-resume", "--no-progress",
		"--help", "--version", "50", "1500", "1-5", "300", "2"}
	helps := map[string]string{}
	for _, h := range []string{"-h", "--help", "-help"} {
		t.Run("help/"+h, func(t *testing.T) {
			code, out, errs, _ := runCLI(t, []string{h}, failReader{t}, noUI(t))
			if code != 0 {
				t.Errorf("exit = %d, want 0", code)
			}
			if errs != "" {
				t.Errorf("stderr = %q, want empty", errs)
			}
			for _, s := range want {
				if !strings.Contains(out, s) {
					t.Errorf("stdout lacks %q", s)
				}
			}
			helps[h] = out
		})
	}
	if helps["-h"] != helps["--help"] {
		t.Errorf("-h and --help outputs differ:\n%q\n%q", helps["-h"], helps["--help"])
	}
	re := regexp.MustCompile(`^fastread \S+\n$`)
	for _, v := range []string{"--version", "-version"} {
		t.Run("version/"+v, func(t *testing.T) {
			code, out, errs, _ := runCLI(t, []string{v}, failReader{t}, noUI(t))
			if code != 0 {
				t.Errorf("exit = %d, want 0", code)
			}
			if errs != "" {
				t.Errorf("stderr = %q, want empty", errs)
			}
			if !re.MatchString(out) {
				t.Errorf("stdout = %q, want %s", out, re)
			}
		})
	}
}
