package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/input"
	"github.com/0x6c6d/fastread/internal/tui"
)

// errCase is one runtime-error input: args and stdin for run/runErr.
type errCase struct {
	name  string
	args  []string
	stdin string
	want  error
}

var listedSentinels = []error{
	input.ErrNotFound, input.ErrCorruptEPUB, input.ErrEmpty, input.ErrUnsupported,
	input.ErrNoText, input.ErrCorruptPDF, input.ErrTooLarge, input.ErrNoInput,
}

// errCases builds the TestErrorsIsChain inputs with their files in a temp directory.
func errCases(t *testing.T) []errCase {
	t.Helper()
	dir := t.TempDir()
	epub, err := os.ReadFile("../../internal/input/testdata/sample.epub")
	if err != nil {
		t.Fatal(err)
	}
	half := filepath.Join(dir, "half.epub")
	if err := os.WriteFile(half, epub[:len(epub)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	xpdf := filepath.Join(dir, "x.pdf")
	if err := os.WriteFile(xpdf, []byte("hello world\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	return []errCase{
		{"missing file", []string{filepath.Join(dir, "nope.txt")}, "", input.ErrNotFound},
		{"truncated epub", []string{half}, "", input.ErrCorruptEPUB},
		{"blank raw text", []string{"   "}, "", input.ErrEmpty},
		{"blank stdin", nil, "\n\n", input.ErrEmpty},
		{"pdf with text content", []string{xpdf}, "", input.ErrUnsupported},
		{"directory", []string{sub}, "", input.ErrUnsupported},
		{"scanned pdf", []string{"../../internal/input/testdata/notext.pdf"}, "", input.ErrNoText},
	}
}

// TestErrorsIsChain is AC27: the error run acts on keeps the input sentinel in its chain.
func TestErrorsIsChain(t *testing.T) {
	for _, tc := range errCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			seams(t, false, func() (tui.Terminal, error) { return newFakeTerm(), nil })
			var out, errb bytes.Buffer
			code, err := runErr(tc.args, strings.NewReader(tc.stdin), &out, &errb, noenv)
			if code != 1 {
				t.Fatalf("code = %d, want 1 (err %v)", code, err)
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("errors.Is(%q, %q) = false", err, tc.want)
			}
			for _, other := range listedSentinels {
				if other != tc.want && errors.Is(err, other) {
					t.Errorf("errors.Is(%q, %q) = true, want false", err, other)
				}
			}
			if out.Len() != 0 || errb.Len() != 0 {
				t.Errorf("runErr printed stdout %q stderr %q", out.String(), errb.String())
			}

			out.Reset()
			errb.Reset()
			if c := run(tc.args, strings.NewReader(tc.stdin), &out, &errb, noenv); c != 1 {
				t.Errorf("run code = %d, want 1", c)
			}
			s := errb.String()
			if lines(s) != 1 || !strings.HasPrefix(s, "fastread: ") || !strings.Contains(s, tc.want.Error()) {
				t.Errorf("run stderr = %q, want one fastread: line containing %q", s, tc.want.Error())
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	type row struct {
		name string
		err  error
		want int
	}
	rows := []row{
		{"nil", nil, 0},
		{"usage error", usageError{errors.New("bad flag")}, 2},
		{"wrapped usage error", fmt.Errorf("x: %w", usageError{errors.New("bad flag")}), 2},
		{"plain error", errors.New("boom"), 1},
	}
	for _, s := range listedSentinels {
		want := 1
		if s == input.ErrNoInput {
			want = 2
		}
		rows = append(rows, row{s.Error(), fmt.Errorf("a: %w", fmt.Errorf("a: %w", s)), want})
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCode(tc.err); got != tc.want {
				t.Errorf("exitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// TestExitCodes is R12: the exit code and stderr shape of run for each class of outcome.
func TestExitCodes(t *testing.T) {
	type row struct {
		name  string
		args  []string
		stdin string
		tty   bool
		open  func() (tui.Terminal, error)
		want  int
		out   bool // stdout may be non-empty (help/version)
	}
	fake := func() (tui.Terminal, error) { return newFakeTerm(), nil }
	failOpen := func() (tui.Terminal, error) { return nil, errors.New("open /dev/tty: no such device") }
	rows := []row{
		{name: "help", args: []string{"--help"}, open: fake, want: 0, out: true},
		{name: "version", args: []string{"--version"}, open: fake, want: 0, out: true},
		{name: "plays to end", args: []string{"--wpm", "1500", "a b"}, open: fake, want: 0},
		{name: "terminal fails", args: []string{"a b"}, open: failOpen, want: 1},
		{name: "gui without display", args: []string{"--ui", "gui", "x"}, open: fake, want: 1},
		{name: "wpm too low", args: []string{"--wpm", "49", "x"}, open: fake, want: 2},
		{name: "unknown flag", args: []string{"--bogus", "x"}, open: fake, want: 2},
		{name: "two positionals", args: []string{"a", "b"}, open: fake, want: 2},
		{name: "start beyond", args: []string{"--start", "5", "a b"}, open: fake, want: 2},
		{name: "no input on tty", tty: true, open: fake, want: 2},
	}
	for _, ec := range errCases(t) {
		rows = append(rows, row{name: "runtime " + ec.name, args: ec.args, stdin: ec.stdin, open: fake, want: 1})
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			_, exited := seams(t, tc.tty, tc.open)
			var out, errb bytes.Buffer
			var stdin io.Reader = strings.NewReader(tc.stdin)
			code := run(tc.args, stdin, &out, &errb, noenv)
			if code != tc.want {
				t.Fatalf("code = %d, want %d (stderr %q)", code, tc.want, errb.String())
			}
			if len(*exited) != 0 {
				t.Errorf("exitProcess called with %v", *exited)
			}
			s := errb.String()
			switch tc.want {
			case 0:
				if s != "" {
					t.Errorf("stderr = %q, want empty", s)
				}
			case 1:
				if lines(s) != 1 || !strings.HasPrefix(s, "fastread: ") {
					t.Errorf("stderr = %q, want exactly one fastread: line", s)
				}
			case 2:
				first, rest, _ := strings.Cut(s, "\n")
				if !strings.HasPrefix(first, "fastread: ") || !strings.Contains(rest, "Usage:") {
					t.Errorf("stderr = %q, want fastread: line followed by usage", s)
				}
			}
			if tc.out != (out.Len() != 0) {
				t.Errorf("stdout = %q, want non-empty only for help/version", out.String())
			}
		})
	}
}
