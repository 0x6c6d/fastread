package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/state"
	"github.com/0x6c6d/fastread/internal/tui"
)

// resumeTerm is a minimal tui.Terminal: Read blocks until Close, Write is recorded and may
// call onWrite with the text so far (escape sequences removed), Size is 80x24.
type resumeTerm struct {
	once    sync.Once
	closed  chan struct{}
	mu      sync.Mutex
	out     bytes.Buffer
	onWrite func(text string)
}

func newResumeTerm() *resumeTerm { return &resumeTerm{closed: make(chan struct{})} }

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]")

func (f *resumeTerm) Read(p []byte) (int, error) {
	<-f.closed
	return 0, io.EOF
}
func (f *resumeTerm) Write(p []byte) (int, error) {
	f.mu.Lock()
	f.out.Write(p)
	text := ansiRE.ReplaceAllString(f.out.String(), "")
	cb := f.onWrite
	f.mu.Unlock()
	if cb != nil {
		cb(text)
	}
	return len(p), nil
}
func (f *resumeTerm) Size() (w, h int, err error) { return 80, 24, nil }
func (f *resumeTerm) MakeRaw() error              { return nil }
func (f *resumeTerm) Restore() error              { return nil }
func (f *resumeTerm) Close() error                { f.once.Do(func() { close(f.closed) }); return nil }

// fakeTUI controls what the replaced runTUI does.
type fakeTUI struct {
	calls   int
	entered int // p.Index() on entry of the last call
	last    int // returned index; -1 = play to end
	err     error
}

// playToEnd starts the player and ticks it until it reports the end.
func playToEnd(p *state.Player) int {
	p.Start()
	for !p.Tick() {
		time.Sleep(time.Until(p.Deadline()))
	}
	return p.Index()
}

// resumeSeams sets all run seams and restores them on cleanup. A nil open makes
// openTerminal return a fresh resumeTerm. A nil ft keeps the real tui.Run.
func resumeSeams(t *testing.T, stdinTTY bool, open func() (tui.Terminal, error), ft *fakeTUI) {
	t.Helper()
	oo, ot, oe, orun, onc := openTerminal, stdinIsTTY, exitProcess, runTUI, notifyContext
	t.Cleanup(func() {
		openTerminal, stdinIsTTY, exitProcess, runTUI, notifyContext = oo, ot, oe, orun, onc
	})
	if open == nil {
		open = func() (tui.Terminal, error) { return newResumeTerm(), nil }
	}
	openTerminal = open
	stdinIsTTY = func(io.Reader) bool { return stdinTTY }
	exitProcess = func(c int) { t.Errorf("exitProcess(%d) called", c) }
	if ft != nil {
		runTUI = func(ctx context.Context, term tui.Terminal, p *state.Player, _ tui.Options) (int, error) {
			defer term.Close()
			ft.calls++
			ft.entered = p.Index()
			if ft.last < 0 {
				return playToEnd(p), ft.err
			}
			return ft.last, ft.err
		}
	}
}

// envOf returns a getenv reading only m.
func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// tmpEnv returns a getenv with XDG_STATE_HOME=<tmp>/state and HOME=<tmp>/home.
func tmpEnv(tmp string) func(string) string {
	return envOf(map[string]string{
		"XDG_STATE_HOME": filepath.Join(tmp, "state"),
		"HOME":           filepath.Join(tmp, "home"),
	})
}

func TestRunRawNoState(t *testing.T) {
	rows := []struct {
		name   string
		args   []string
		stdin  string
		last   int
		noEnv  bool
		wantAt int
	}{
		{"raw quit at 1", []string{"--wpm", "1500", "a b c"}, "", 1, false, 0},
		{"raw to end", []string{"--wpm", "1500", "a b c"}, "", -1, false, 0},
		{"stdin quit at 1", []string{"--wpm", "1500"}, "a b c", 1, false, 0},
		{"stdin to end", []string{"--wpm", "1500"}, "a b c", -1, false, 0},
		{"raw no-resume", []string{"--wpm", "1500", "--no-resume", "a b c"}, "", 1, false, 0},
		{"raw start 1", []string{"--wpm", "1500", "--start", "1", "a b c"}, "", 2, false, 1},
		{"raw empty env", []string{"--wpm", "1500", "a b c"}, "", 1, true, 0},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			ft := &fakeTUI{last: tc.last}
			resumeSeams(t, false, nil, ft)
			getenv := tmpEnv(tmp)
			if tc.noEnv {
				getenv = envOf(nil)
			}
			var out, errb bytes.Buffer
			code := run(tc.args, strings.NewReader(tc.stdin), &out, &errb, getenv)
			if code != 0 || errb.Len() != 0 {
				t.Fatalf("code = %d, stderr %q; want 0 and empty", code, errb.String())
			}
			if ft.calls != 1 || ft.entered != tc.wantAt {
				t.Errorf("runTUI calls %d, started at %d; want 1, %d", ft.calls, ft.entered, tc.wantAt)
			}
			for _, d := range []string{"state", "home"} {
				if _, err := os.Stat(filepath.Join(tmp, d)); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("%s exists after run (err %v)", d, err)
				}
			}
		})
	}
}

// writeBook writes w00..w19 to <dir>/book.txt and returns its path.
func writeBook(t *testing.T, dir string) string {
	t.Helper()
	words := make([]string, 20)
	for i := range words {
		words[i] = fmt.Sprintf("w%02d", i)
	}
	p := filepath.Join(dir, "book.txt")
	if err := os.WriteFile(p, []byte(strings.Join(words, " ")), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// entryOf returns the resume entry path for book under <tmp>/state.
func entryOf(t *testing.T, tmp, book string) string {
	t.Helper()
	abs, err := filepath.Abs(book)
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatal(err)
	}
	return state.NewStore(filepath.Join(tmp, "state", "fastread")).EntryPath(real)
}

// entryIndex reads the JSON index of the entry, failing the test if it is missing.
func entryIndex(t *testing.T, entry string) int {
	t.Helper()
	b, err := os.ReadFile(entry)
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	var e struct {
		Index *int `json:"index"`
	}
	if err := json.Unmarshal(b, &e); err != nil || e.Index == nil {
		t.Fatalf("entry %q: %v", b, err)
	}
	return *e.Index
}

func TestRunResumeFile(t *testing.T) {
	tmp := t.TempDir()
	book := writeBook(t, tmp)
	entry := entryOf(t, tmp, book)
	env := tmpEnv(tmp)

	type step struct {
		name      string
		args      []string
		before    func(t *testing.T)
		last      int
		err       error
		wantStart int
		wantCode  int
		wantWarn  bool   // exactly one stderr line containing "warning"
		wantErr   string // exactly one stderr line containing this
		after     func(t *testing.T)
	}
	steps := []step{
		{name: "first run", args: []string{book}, last: 7, wantStart: 0, after: func(t *testing.T) {
			fi, err := os.Stat(entry)
			if err != nil {
				t.Fatal(err)
			}
			di, err := os.Stat(filepath.Dir(entry))
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
				t.Errorf("modes entry %v dir %v, want 0600 0700", fi.Mode().Perm(), di.Mode().Perm())
			}
			if got := entryIndex(t, entry); got != 7 {
				t.Errorf("index = %d, want 7", got)
			}
			if b, _ := os.ReadFile(entry); bytes.Contains(b, []byte("w0")) {
				t.Errorf("entry contains text: %q", b)
			}
		}},
		{name: "resumes", args: []string{book}, last: 7, wantStart: 7},
		{name: "start overrides", args: []string{"--start", "3", book}, last: 4, wantStart: 3, after: func(t *testing.T) {
			if got := entryIndex(t, entry); got != 4 {
				t.Errorf("index = %d, want 4", got)
			}
		}},
		{name: "no-resume", args: []string{"--no-resume", book}, last: 2, wantStart: 0, after: func(t *testing.T) {
			if got := entryIndex(t, entry); got != 2 {
				t.Errorf("index = %d, want 2", got)
			}
		}},
		{name: "file changed", args: []string{book}, last: 3, wantStart: 0, before: func(t *testing.T) {
			f, err := os.OpenFile(book, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteString(" w20"); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "end deletes", args: []string{"--wpm", "1500", "--start", "18", book}, last: -1, wantStart: 18, after: func(t *testing.T) {
			if _, err := os.Stat(entry); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("entry still exists (err %v)", err)
			}
		}},
		{name: "corrupt entry", args: []string{book}, last: 6, wantStart: 0, wantWarn: true, before: func(t *testing.T) {
			if err := os.WriteFile(entry, []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, after: func(t *testing.T) {
			if got := entryIndex(t, entry); got != 6 {
				t.Errorf("index = %d, want 6", got)
			}
		}},
		{name: "tui error still saves", args: []string{book}, last: 5, err: errors.New("boom"), wantStart: 6, wantCode: 1, wantErr: "boom", after: func(t *testing.T) {
			if got := entryIndex(t, entry); got != 5 {
				t.Errorf("index = %d, want 5", got)
			}
		}},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			if s.before != nil {
				s.before(t)
			}
			ft := &fakeTUI{last: s.last, err: s.err}
			resumeSeams(t, false, nil, ft)
			var out, errb bytes.Buffer
			code := run(s.args, strings.NewReader(""), &out, &errb, env)
			if code != s.wantCode {
				t.Fatalf("code = %d, want %d (stderr %q)", code, s.wantCode, errb.String())
			}
			if ft.entered != s.wantStart {
				t.Errorf("started at %d, want %d", ft.entered, s.wantStart)
			}
			es := errb.String()
			switch {
			case s.wantWarn:
				if lines(es) != 1 || !strings.Contains(es, "warning") {
					t.Errorf("stderr = %q, want one warning line", es)
				}
			case s.wantErr != "":
				if lines(es) != 1 || !strings.Contains(es, s.wantErr) {
					t.Errorf("stderr = %q, want one line containing %q", es, s.wantErr)
				}
			default:
				if es != "" {
					t.Errorf("stderr = %q, want empty", es)
				}
			}
			if s.after != nil {
				s.after(t)
			}
		})
	}

	t.Run("state dir below a file", func(t *testing.T) {
		blocker := filepath.Join(tmp, "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		resumeSeams(t, false, nil, &fakeTUI{last: 2})
		var out, errb bytes.Buffer
		code := run([]string{"--no-resume", book}, strings.NewReader(""), &out, &errb,
			envOf(map[string]string{"XDG_STATE_HOME": filepath.Join(blocker, "x")}))
		es := errb.String()
		if code != 1 || lines(es) != 1 || !strings.HasPrefix(es, "fastread: ") ||
			!strings.Contains(es, "saving reading position") {
			t.Errorf("code %d stderr %q, want 1 and one fastread: ...saving reading position... line", code, es)
		}
	})

	t.Run("no state dir", func(t *testing.T) {
		ft := &fakeTUI{last: 2}
		resumeSeams(t, false, nil, ft)
		var out, errb bytes.Buffer
		code := run([]string{filepath.Join(tmp, "book.txt")}, strings.NewReader(""), &out, &errb, envOf(nil))
		es := errb.String()
		if code != 0 || lines(es) != 1 || !strings.Contains(es, "resume disabled") {
			t.Errorf("code %d stderr %q, want 0 and one resume disabled line", code, es)
		}
		if ft.entered != 0 {
			t.Errorf("started at %d, want 0", ft.entered)
		}
		if ents, _ := os.ReadDir(tmp); len(ents) != 3 { // book.txt, state/, blocker
			t.Errorf("tmp holds %v, want book.txt, blocker, state", ents)
		}
	})

	t.Run("terminal fails", func(t *testing.T) {
		before, err := os.ReadFile(entry)
		if err != nil {
			t.Fatal(err)
		}
		ft := &fakeTUI{last: 9}
		resumeSeams(t, false, func() (tui.Terminal, error) { return nil, errors.New("no tty") }, ft)
		var out, errb bytes.Buffer
		code := run([]string{book}, strings.NewReader(""), &out, &errb, env)
		if code != 1 || ft.calls != 0 {
			t.Errorf("code %d, runTUI calls %d; want 1, 0", code, ft.calls)
		}
		after, err := os.ReadFile(entry)
		if err != nil || !bytes.Equal(before, after) {
			t.Errorf("entry changed: %q -> %q (err %v)", before, after, err)
		}
	})
}

func TestRunSignalSaves(t *testing.T) {
	tmp := t.TempDir()
	book := writeBook(t, tmp)
	entry := entryOf(t, tmp, book)

	term := newResumeTerm()
	resumeSeams(t, false, func() (tui.Terminal, error) { return term, nil }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifyContext = func(parent context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
		return ctx, cancel
	}
	term.onWrite = func(text string) {
		if strings.Contains(text, "w00") {
			cancel()
		}
	}

	type result struct {
		code int
		errs string
	}
	done := make(chan result, 1)
	go func() {
		var out, errb bytes.Buffer
		code := run([]string{"--wpm", "50", book}, strings.NewReader(""), &out, &errb, tmpEnv(tmp))
		done <- result{code, errb.String()}
	}()
	select {
	case r := <-done:
		if r.code != 0 || r.errs != "" {
			t.Fatalf("code %d stderr %q, want 0 and empty", r.code, r.errs)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not return within 2 s after the context was cancelled")
	}
	if got := entryIndex(t, entry); got != 0 {
		t.Errorf("index = %d, want 0", got)
	}
}
