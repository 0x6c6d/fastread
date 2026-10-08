package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// exitTerm is a pipe-backed Terminal with fault injection for the exit-path tests.
// Every attempted Write is recorded, also the ones that fail.
type exitTerm struct {
	r *io.PipeReader
	w *io.PipeWriter

	// fault injection, set before Run starts
	makeRawErr   error
	restoreErr   error
	restorePanic bool
	sizeErr      error
	panicSize    bool
	failWriteAt  int // Write calls numbered from 1; this one and all later ones fail (0 = never)

	mu                         sync.Mutex
	writes                     []string
	makeRaws, restores, closes int
}

func newExitTerm() *exitTerm {
	r, w := io.Pipe()
	return &exitTerm{r: r, w: w}
}

func (e *exitTerm) Read(p []byte) (int, error) { return e.r.Read(p) }

func (e *exitTerm) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.writes = append(e.writes, string(p))
	if e.failWriteAt > 0 && len(e.writes) >= e.failWriteAt {
		return 0, errors.New("write boom")
	}
	return len(p), nil
}

func (e *exitTerm) Size() (int, int, error) {
	if e.panicSize {
		panic("size boom")
	}
	if e.sizeErr != nil {
		return 0, 0, e.sizeErr
	}
	return 40, 5, nil
}

func (e *exitTerm) MakeRaw() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.makeRaws++
	return e.makeRawErr
}

func (e *exitTerm) Restore() error {
	e.mu.Lock()
	e.restores++
	e.mu.Unlock()
	if e.restorePanic {
		panic("restore boom")
	}
	return e.restoreErr
}

func (e *exitTerm) Close() error {
	e.mu.Lock()
	e.closes++
	e.mu.Unlock()
	e.r.CloseWithError(errors.New("closed"))
	return nil
}

// key writes s as one read chunk; it returns once Run's reader has consumed it or the
// terminal is closed.
func (e *exitTerm) key(s string) { _, _ = e.w.Write([]byte(s)) }

// hangup makes the next Read return io.EOF, like a terminal whose other end went away.
func (e *exitTerm) hangup() { e.w.Close() }

func (e *exitTerm) output() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return strings.Join(e.writes, "")
}

func (e *exitTerm) nWrites() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.writes)
}

func (e *exitTerm) counts() (mk, rs, cl int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.makeRaws, e.restores, e.closes
}

// waitWrites polls until at least n writes happened (or 2 s passed).
func (e *exitTerm) waitWrites(n int) {
	deadline := time.Now().Add(2 * time.Second)
	for e.nWrites() < n && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}

// exitResizeTerm adds Resizer to exitTerm.
type exitResizeTerm struct {
	*exitTerm
	ch chan struct{}
}

func (r *exitResizeTerm) Resized() <-chan struct{} { return r.ch }

// exitRow is one exit path of Run.
type exitRow struct {
	name     string
	words    int
	wpm      int
	prep     func(e *exitTerm) // fault injection before Run
	seam     func() func()     // replaces a seam; returns the undo
	act      func(e *exitTerm, cancel context.CancelFunc)
	wantLast int
	wantErr  string // "" = nil error, else a substring of the error
	restores int    // RestoreSeq count in the output
}

func panicOnCall(n int) func() func() {
	return func() func() {
		old := renderFn
		var mu sync.Mutex
		calls := 0
		renderFn = func(m Model, w, h int) Frame {
			mu.Lock()
			calls++
			c := calls
			mu.Unlock()
			if c == n {
				panic(fmt.Sprintf("render boom %d", n))
			}
			return old(m, w, h)
		}
		return func() { renderFn = old }
	}
}

func exitRows() []exitRow {
	afterFrame := func(f func(e *exitTerm, cancel context.CancelFunc)) func(*exitTerm, context.CancelFunc) {
		return func(e *exitTerm, cancel context.CancelFunc) {
			e.waitWrites(2) // EnterSeq and the first frame
			f(e, cancel)
		}
	}
	return []exitRow{
		{name: "q", act: afterFrame(func(e *exitTerm, _ context.CancelFunc) { e.key("q") }), restores: 1},
		{name: "ctrl-c", act: afterFrame(func(e *exitTerm, _ context.CancelFunc) { e.key("\x03") }), restores: 1},
		{name: "lone esc", act: afterFrame(func(e *exitTerm, _ context.CancelFunc) { e.key("\x1b") }), restores: 1},
		{name: "ctx cancelled", act: afterFrame(func(_ *exitTerm, cancel context.CancelFunc) { cancel() }), restores: 1},
		{name: "end of text", words: 3, wpm: 3000, wantLast: 2, restores: 1},
		{name: "panic in render", seam: panicOnCall(1), wantErr: "tui: internal error: render boom 1", restores: 1},
		{name: "panic in Size", prep: func(e *exitTerm) { e.panicSize = true }, wantErr: "tui: internal error: size boom", restores: 1},
		{name: "write error on enter", prep: func(e *exitTerm) { e.failWriteAt = 1 }, wantErr: "tui: writing frame: write boom", restores: 1},
		{name: "write error on frame", prep: func(e *exitTerm) { e.failWriteAt = 2 }, wantErr: "tui: writing frame: write boom", restores: 1},
		{name: "read EOF", act: afterFrame(func(e *exitTerm, _ context.CancelFunc) { e.hangup() }), wantErr: "tui: reading keys: EOF", restores: 1},
		{name: "size error", prep: func(e *exitTerm) { e.sizeErr = errors.New("size gone") }, wantErr: "tui: terminal size: size gone", restores: 1},
		{name: "MakeRaw error", prep: func(e *exitTerm) { e.makeRawErr = errors.New("no raw") }, wantErr: "tui: raw mode: no raw", restores: 0},
	}
}

// runExitRow runs one row and checks its outcome; it returns the terminal for further checks.
func runExitRow(t *testing.T, row exitRow, resizer bool) *exitTerm {
	t.Helper()
	e := newExitTerm()
	if row.prep != nil {
		row.prep(e)
	}
	if row.seam != nil {
		undo := row.seam()
		defer undo()
	}
	var term Terminal = e
	if resizer {
		term = &exitResizeTerm{exitTerm: e, ch: make(chan struct{}, 1)}
	}
	words, wpm := row.words, row.wpm
	if words == 0 {
		words = 5
	}
	if wpm == 0 {
		wpm = 60
	}
	ws := make([]string, words)
	for i := range ws {
		ws[i] = fmt.Sprintf("w%02d", i)
	}
	p := newPlayer(wpm, ws...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan runResult, 1)
	start := time.Now()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- runResult{-1, fmt.Errorf("Run panicked: %v", r)}
			}
		}()
		last, err := Run(ctx, term, p, Options{Getenv: noEnv})
		ch <- runResult{last, err}
	}()
	if row.act != nil {
		go row.act(e, cancel)
	}
	var res runResult
	select {
	case res = <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: Run did not return within 2s", row.name)
	}
	el := time.Since(start)

	if res.last != row.wantLast {
		t.Errorf("%s: last = %d, want %d", row.name, res.last, row.wantLast)
	}
	switch {
	case row.wantErr == "" && res.err != nil:
		t.Errorf("%s: err = %v, want nil", row.name, res.err)
	case row.wantErr != "" && (res.err == nil || !strings.Contains(res.err.Error(), row.wantErr)):
		t.Errorf("%s: err = %v, want containing %q", row.name, res.err, row.wantErr)
	}
	out := e.output()
	if n := strings.Count(out, RestoreSeq); n != row.restores {
		t.Errorf("%s: RestoreSeq written %d times, want %d", row.name, n, row.restores)
	}
	if row.restores > 0 && !strings.HasSuffix(out, RestoreSeq) {
		t.Errorf("%s: output does not end with RestoreSeq: %q", row.name, tail(out))
	}
	if row.restores == 0 && out != "" {
		t.Errorf("%s: wrote %q, want nothing", row.name, tail(out))
	}
	mk, rs, cl := e.counts()
	if mk != 1 {
		t.Errorf("%s: MakeRaw called %d times, want 1", row.name, mk)
	}
	if cl < 1 {
		t.Errorf("%s: Close not called", row.name)
	}
	if row.restores > 0 && rs < 1 {
		t.Errorf("%s: Restore not called", row.name)
	}
	t.Logf("%s (resizer=%v): (%d, %v) in %v", row.name, resizer, res.last, res.err, el.Round(time.Millisecond))
	return e
}

func tail(s string) string {
	if len(s) > 60 {
		return "..." + s[len(s)-60:]
	}
	return s
}

func TestLoopExitPaths(t *testing.T) {
	for _, row := range exitRows() {
		for _, rz := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/resizer=%v", row.name, rz), func(t *testing.T) {
				runExitRow(t, row, rz)
			})
		}
	}
}

func TestRestoreOnPanic(t *testing.T) {
	keysThenWait := func(e *exitTerm, _ context.CancelFunc) {
		e.waitWrites(2)
		e.key(" ") // pause: frame 2
		e.key(" ") // resume: frame 3 panics
	}
	encodePanic := func() func() {
		old := encodeFn
		encodeFn = func(Frame, ColorMode) []byte { panic("encode boom") }
		return func() { encodeFn = old }
	}
	hookPanic := func() func() {
		old := frameHook
		frameHook = func(Model, Frame) { panic("hook boom") }
		return func() { frameHook = old }
	}
	rows := []exitRow{
		{name: "render first call", seam: panicOnCall(1), wantErr: "render boom 1"},
		{name: "render third call", seam: panicOnCall(3), act: keysThenWait, wantErr: "render boom 3"},
		{name: "encode", seam: encodePanic, wantErr: "encode boom"},
		{name: "frameHook", seam: hookPanic, wantErr: "hook boom"},
		{name: "render with Restore error", seam: panicOnCall(1), prep: func(e *exitTerm) { e.restoreErr = errors.New("restore failed") }, wantErr: "render boom 1"},
		{name: "render with Restore panic", seam: panicOnCall(1), prep: func(e *exitTerm) { e.restorePanic = true }, wantErr: "render boom 1"},
	}
	for _, row := range rows {
		row.restores = 1
		row.wantErr = "tui: internal error: " + row.wantErr
		t.Run(row.name, func(t *testing.T) {
			runExitRow(t, row, false)
		})
	}
}

func TestLoopNoLeak(t *testing.T) {
	runtime.GC()
	warm := exitRows()[0]
	runExitRow(t, warm, false)
	runtime.GC()
	base := runtime.NumGoroutine()

	runs := 0
	for _, row := range exitRows() {
		for _, rz := range []bool{false, true} {
			for i := 0; i < 3; i++ {
				runExitRow(t, row, rz)
				runs++
			}
		}
	}
	deadline := time.Now().Add(time.Second)
	n := runtime.NumGoroutine()
	for n > base && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		runtime.GC()
		n = runtime.NumGoroutine()
	}
	t.Logf("%d runs; goroutines: baseline %d, after %d", runs, base, n)
	if n > base {
		buf := make([]byte, 1<<16)
		t.Errorf("goroutines: baseline %d, after %d runs %d\n%s", base, runs, n, buf[:runtime.Stack(buf, true)])
	}
}
