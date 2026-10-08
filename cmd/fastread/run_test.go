package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/0x6c6d/fastread/internal/tui"
)

// fakeTerm is a terminal whose Read blocks until Close; Write discards; Size is 80x24.
type fakeTerm struct {
	once   sync.Once
	closed chan struct{}
}

func newFakeTerm() *fakeTerm { return &fakeTerm{closed: make(chan struct{})} }

func (f *fakeTerm) Read(p []byte) (int, error) {
	<-f.closed
	return 0, io.EOF
}
func (f *fakeTerm) Write(p []byte) (int, error) { return len(p), nil }
func (f *fakeTerm) Size() (w, h int, err error) { return 80, 24, nil }
func (f *fakeTerm) MakeRaw() error              { return nil }
func (f *fakeTerm) Restore() error              { return nil }
func (f *fakeTerm) Close() error                { f.once.Do(func() { close(f.closed) }); return nil }
func (f *fakeTerm) isClosed() bool {
	select {
	case <-f.closed:
		return true
	default:
		return false
	}
}

// seams sets the test seams and restores them on cleanup. opened counts openTerminal calls.
func seams(t *testing.T, tty bool, open func() (tui.Terminal, error)) (opened *int, exited *[]int) {
	t.Helper()
	oo, ot, oe := openTerminal, stdinIsTTY, exitProcess
	t.Cleanup(func() { openTerminal, stdinIsTTY, exitProcess = oo, ot, oe })
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	n := 0
	var codes []int
	openTerminal = func() (tui.Terminal, error) { n++; return open() }
	stdinIsTTY = func(io.Reader) bool { return tty }
	exitProcess = func(c int) { codes = append(codes, c) }
	return &n, &codes
}

func noenv(string) string { return "" }

func lines(s string) int { return strings.Count(s, "\n") }

func TestRunEmptyText(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		stdin string
	}{
		{"raw blank", []string{"   "}, ""},
		{"empty stdin", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opened, _ := seams(t, false, func() (tui.Terminal, error) { return newFakeTerm(), nil })
			var out, errb bytes.Buffer
			code := run(tc.args, strings.NewReader(tc.stdin), &out, &errb, noenv)
			if code != 1 {
				t.Fatalf("code = %d, want 1 (stderr %q)", code, errb.String())
			}
			if lines(errb.String()) != 1 || !strings.Contains(errb.String(), "empty text") ||
				!strings.HasPrefix(errb.String(), "fastread: ") {
				t.Errorf("stderr = %q, want one 'fastread: ...empty text' line", errb.String())
			}
			if out.Len() != 0 || *opened != 0 {
				t.Errorf("stdout %q, terminal opened %d times", out.String(), *opened)
			}
		})
	}
}

func TestRunNoInputTTY(t *testing.T) {
	opened, _ := seams(t, true, func() (tui.Terminal, error) { return newFakeTerm(), nil })
	var out, errb bytes.Buffer
	code := run(nil, strings.NewReader(""), &out, &errb, noenv)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.HasPrefix(errb.String(), "fastread: ") || !strings.Contains(errb.String(), "Usage:") {
		t.Errorf("stderr = %q, want error line and usage", errb.String())
	}
	if *opened != 0 {
		t.Errorf("terminal opened")
	}
}

func TestRunStartBeyond(t *testing.T) {
	for _, tc := range []struct {
		start    string
		wantCode int
		wantUI   bool
	}{
		{"2", 2, false},
		{"5", 2, false},
		{"1", 0, true},
		{"0", 0, true},
	} {
		t.Run(tc.start, func(t *testing.T) {
			opened, _ := seams(t, false, func() (tui.Terminal, error) { return newFakeTerm(), nil })
			var out, errb bytes.Buffer
			code := run([]string{"--wpm", "1500", "--start", tc.start, "a b"}, strings.NewReader(""), &out, &errb, noenv)
			if code != tc.wantCode {
				t.Fatalf("code = %d, want %d (stderr %q)", code, tc.wantCode, errb.String())
			}
			if (*opened == 1) != tc.wantUI {
				t.Errorf("terminal opened %d times, want UI reached = %v", *opened, tc.wantUI)
			}
			if tc.wantCode == 2 && !strings.Contains(errb.String(), "Usage:") {
				t.Errorf("stderr = %q, want usage", errb.String())
			}
			if tc.wantCode == 0 && errb.Len() != 0 {
				t.Errorf("stderr = %q, want empty", errb.String())
			}
		})
	}
}

func TestRunTerminalError(t *testing.T) {
	seams(t, false, func() (tui.Terminal, error) {
		return nil, errors.New("tui: open /dev/tty: no such device\nor address")
	})
	var out, errb bytes.Buffer
	code := run([]string{"hello world"}, strings.NewReader(""), &out, &errb, noenv)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if lines(errb.String()) != 1 || !strings.HasPrefix(errb.String(), "fastread: ") {
		t.Errorf("stderr = %q, want exactly one fastread: line", errb.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestRunTUIPlaysToEnd(t *testing.T) {
	ft := newFakeTerm()
	opened, exited := seams(t, false, func() (tui.Terminal, error) { return ft, nil })
	var out, errb bytes.Buffer
	code := run([]string{"--wpm", "1500", "a b c"}, strings.NewReader(""), &out, &errb, noenv)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr %q)", code, errb.String())
	}
	if errb.Len() != 0 || out.Len() != 0 {
		t.Errorf("stdout %q stderr %q, want both empty", out.String(), errb.String())
	}
	if *opened != 1 || !ft.isClosed() || len(*exited) != 0 {
		t.Errorf("opened %d, closed %v, exitProcess calls %v", *opened, ft.isClosed(), *exited)
	}
}

func TestRunGUINoDisplay(t *testing.T) {
	opened, exited := seams(t, false, func() (tui.Terminal, error) { return newFakeTerm(), nil })
	var out, errb bytes.Buffer
	code := run([]string{"--ui", "gui", "x"}, strings.NewReader(""), &out, &errb, noenv)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if lines(errb.String()) != 1 || !strings.HasPrefix(errb.String(), "fastread: ") {
		t.Errorf("stderr = %q, want exactly one fastread: line", errb.String())
	}
	if len(*exited) != 0 || *opened != 0 || out.Len() != 0 {
		t.Errorf("exitProcess calls %v, terminal opened %d, stdout %q", *exited, *opened, out.String())
	}
}
