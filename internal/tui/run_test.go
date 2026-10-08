package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/state"
	"github.com/0x6c6d/fastread/internal/tokenize"
)

// fakeTerm is a pipe-backed Terminal for tests.
type fakeTerm struct {
	r *io.PipeReader
	w *io.PipeWriter

	w0, h0     int
	panicSize  bool
	makeRawErr error
	writeDelay time.Duration // slept on every Write call (outside the lock)

	mu                         sync.Mutex
	out                        bytes.Buffer
	makeRaws, restores, closes int
}

func newFakeTerm() *fakeTerm {
	r, w := io.Pipe()
	return &fakeTerm{r: r, w: w, w0: 40, h0: 5}
}

func (f *fakeTerm) Read(p []byte) (int, error) { return f.r.Read(p) }

func (f *fakeTerm) Write(p []byte) (int, error) {
	if f.writeDelay > 0 {
		time.Sleep(f.writeDelay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.out.Write(p)
}

func (f *fakeTerm) Size() (int, int, error) {
	if f.panicSize {
		panic("size boom")
	}
	return f.w0, f.h0, nil
}

func (f *fakeTerm) MakeRaw() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.makeRaws++
	return f.makeRawErr
}

func (f *fakeTerm) Restore() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restores++
	return nil
}

func (f *fakeTerm) Close() error {
	f.mu.Lock()
	f.closes++
	f.mu.Unlock()
	f.r.CloseWithError(errors.New("closed"))
	return nil
}

func (f *fakeTerm) output() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.out.String()
}

func (f *fakeTerm) counts() (mk, rs, cl int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.makeRaws, f.restores, f.closes
}

// sendKeys writes keys into the terminal's input from a goroutine; it ends when the
// bytes are read or the terminal is closed.
func (f *fakeTerm) sendKeys(s string) {
	go func() { _, _ = f.w.Write([]byte(s)) }()
}

func newPlayer(wpm int, words ...string) *state.Player {
	toks := make([]tokenize.Token, len(words))
	for i, w := range words {
		toks[i] = tokenize.Token{Text: w}
	}
	return state.NewPlayer(state.Config{Tokens: toks, WPM: wpm, Size: 1})
}

type runResult struct {
	last int
	err  error
}

// runBounded runs Run in a goroutine and fails the test if it takes longer than 5 s.
func runBounded(t *testing.T, ctx context.Context, ft *fakeTerm, p *state.Player, opts Options) (int, error) {
	t.Helper()
	ch := make(chan runResult, 1)
	go func() {
		last, err := Run(ctx, ft, p, opts)
		ch <- runResult{last, err}
	}()
	select {
	case r := <-ch:
		return r.last, r.err
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s")
		return 0, nil
	}
}

func noEnv(string) string { return "" }

func TestRunQuitRestores(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"q", "q"},
		{"ctrl-c", "\x03"},
		{"esc", "\x1b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ft := newFakeTerm()
			p := newPlayer(50, "hello", "world")
			ft.sendKeys(tc.key)
			last, err := runBounded(t, context.Background(), ft, p, Options{Getenv: noEnv})
			if last != 0 || err != nil {
				t.Fatalf("Run = (%d, %v), want (0, nil)", last, err)
			}
			out := ft.output()
			if !strings.HasPrefix(out, EnterSeq) {
				t.Errorf("output does not start with EnterSeq: %q", out)
			}
			if !strings.HasSuffix(out, RestoreSeq) {
				t.Errorf("output does not end with RestoreSeq: %q", out)
			}
			mk, rs, cl := ft.counts()
			if mk < 1 || rs < 1 || cl < 1 {
				t.Errorf("MakeRaw/Restore/Close = %d/%d/%d, want each >= 1", mk, rs, cl)
			}
		})
	}
}

func TestRunPlaysToEnd(t *testing.T) {
	ft := newFakeTerm()
	p := newPlayer(1500, "alpha", "bravo", "charlie")
	env := func(k string) string {
		if k == "COLORTERM" {
			return "truecolor"
		}
		return ""
	}
	start := time.Now()
	last, err := runBounded(t, context.Background(), ft, p, Options{Getenv: env})
	if el := time.Since(start); el > 2*time.Second {
		t.Errorf("took %v, want < 2s", el)
	}
	if last != 2 || err != nil {
		t.Fatalf("Run = (%d, %v), want (2, nil)", last, err)
	}
	if !p.Finished() {
		t.Error("p.Finished() = false, want true")
	}
	out := ft.output()
	const red = "\x1b[38;2;255;0;0m"
	for _, l := range []string{"l", "r", "a"} {
		if !strings.Contains(out, red+l) {
			t.Errorf("output lacks red focus letter %q", l)
		}
	}
	if !strings.HasSuffix(out, RestoreSeq) {
		t.Error("output does not end with RestoreSeq")
	}
}

func TestRunCtxCancel(t *testing.T) {
	ft := newFakeTerm()
	p := newPlayer(50, "hello", "world")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			if len(ft.output()) > len(EnterSeq) {
				break
			}
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	start := time.Now()
	_, err := runBounded(t, ctx, ft, p, Options{Getenv: noEnv})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if el := time.Since(start); el > time.Second {
		t.Errorf("took %v, want quick return", el)
	}
	if p.Finished() {
		t.Error("p.Finished() = true, want false")
	}
	if !strings.HasSuffix(ft.output(), RestoreSeq) {
		t.Error("output does not end with RestoreSeq")
	}
}

func TestRunPanicRestores(t *testing.T) {
	ft := newFakeTerm()
	ft.panicSize = true
	p := newPlayer(50, "hello")
	_, err := runBounded(t, context.Background(), ft, p, Options{Getenv: noEnv})
	if err == nil {
		t.Fatal("Run err = nil, want internal error")
	}
	if !strings.Contains(err.Error(), "tui: internal error: size boom") {
		t.Errorf("err = %v", err)
	}
	if !strings.HasSuffix(ft.output(), RestoreSeq) {
		t.Error("output does not end with RestoreSeq")
	}
	_, rs, cl := ft.counts()
	if rs < 1 || cl < 1 {
		t.Errorf("Restore/Close = %d/%d, want each >= 1", rs, cl)
	}
}

func TestRunMakeRawFails(t *testing.T) {
	ft := newFakeTerm()
	ft.makeRawErr = errors.New("no raw")
	p := newPlayer(50, "hello")
	_, err := runBounded(t, context.Background(), ft, p, Options{Getenv: noEnv})
	if err == nil {
		t.Fatal("Run err = nil, want error")
	}
	if n := len(ft.output()); n != 0 {
		t.Errorf("wrote %d bytes, want 0", n)
	}
	if _, _, cl := ft.counts(); cl < 1 {
		t.Error("Close not called")
	}
}

func TestRunNoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()
	ft := newFakeTerm()
	p := newPlayer(50, "hello", "world")
	ft.sendKeys("q")
	if _, err := runBounded(t, context.Background(), ft, p, Options{Getenv: noEnv}); err != nil {
		t.Fatalf("Run err = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	after := runtime.NumGoroutine()
	for after > before && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		after = runtime.NumGoroutine()
	}
	if after > before {
		t.Errorf("goroutines: %d before, %d after", before, after)
	}
}
