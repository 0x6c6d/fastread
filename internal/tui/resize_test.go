package tui

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/state"
	"github.com/0x6c6d/fastread/internal/tokenize"
	"github.com/0x6c6d/fastread/internal/tui/glyph"
)

// resizeTerm is a fakeTerm that also implements Resizer with a settable size.
type resizeTerm struct {
	*fakeTerm
	ch chan struct{}

	smu     sync.Mutex
	w, h    int
	sizeErr error
}

func newResizeTerm(w, h int) *resizeTerm {
	return &resizeTerm{fakeTerm: newFakeTerm(), ch: make(chan struct{}, 1), w: w, h: h}
}

func (r *resizeTerm) Size() (int, int, error) {
	r.smu.Lock()
	defer r.smu.Unlock()
	return r.w, r.h, r.sizeErr
}

func (r *resizeTerm) Resized() <-chan struct{} { return r.ch }

// resize sets the size and signals (coalesced, never blocking).
func (r *resizeTerm) resize(w, h int) {
	r.smu.Lock()
	r.w, r.h = w, h
	r.smu.Unlock()
	r.signal()
}

func (r *resizeTerm) signal() {
	select {
	case r.ch <- struct{}{}:
	default:
	}
}

// resizeRec records hooked frames and invariant violations.
type resizeRec struct {
	mu     sync.Mutex
	frames []frameRec
	errs   []string
}

func (r *resizeRec) all() []frameRec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]frameRec(nil), r.frames...)
}

func (r *resizeRec) errors() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.errs...)
}

func hookResize(t *testing.T, p *state.Player) *resizeRec {
	t.Helper()
	r := &resizeRec{}
	old := frameHook
	t.Cleanup(func() { frameHook = old })
	frameHook = func(m Model, f Frame) {
		var e string
		switch {
		case f.Size == 0:
			if f.Parts != 0 {
				e = fmt.Sprintf("too-small frame %dx%d has Parts %d", f.W, f.H, f.Parts)
			}
		case m.Part >= f.Parts || p.Parts() != f.Parts:
			e = fmt.Sprintf("frame %dx%d: Part %d, f.Parts %d, p.Parts() %d", f.W, f.H, m.Part, f.Parts, p.Parts())
		}
		r.mu.Lock()
		r.frames = append(r.frames, frameRec{m: m, f: f, at: time.Now()})
		if e != "" {
			r.errs = append(r.errs, e)
		}
		r.mu.Unlock()
	}
	return r
}

// waitSized waits for a frame from index from on with size w x h and model condition ok.
func waitSized(t *testing.T, r *resizeRec, from, w, h int, ok func(Model) bool) (frameRec, int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		fs := r.all()
		for i := from; i < len(fs); i++ {
			if fs[i].f.W == w && fs[i].f.H == h && (ok == nil || ok(fs[i].m)) {
				return fs[i], len(fs)
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no frame of size %dx%d (frames %d)", w, h, len(fs))
		}
		time.Sleep(time.Millisecond)
	}
}

// focusCol returns the measured focus column of a block frame, or -1.
func focusCol(f Frame) int {
	minX := -1
	for _, row := range f.Cells {
		for x, c := range row {
			if c.Style == StyleFocus && (minX < 0 || x < minX) {
				minX = x
			}
		}
	}
	if minX < 0 {
		return -1
	}
	return minX + glyph.Width(f.Size)/2
}

// frameText returns the non-blank rows of f, trimmed, joined by newlines.
func frameText(f Frame) string {
	var rows []string
	for _, row := range f.Cells {
		var b strings.Builder
		for _, c := range row {
			if c.Text == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(c.Text)
			}
		}
		if s := strings.TrimSpace(b.String()); s != "" {
			rows = append(rows, s)
		}
	}
	return strings.Join(rows, "\n")
}

func startResizeRun(ctx context.Context, rt *resizeTerm, p *state.Player) <-chan runResult {
	ch := make(chan runResult, 1)
	go func() {
		last, err := Run(ctx, rt, p, Options{Getenv: noEnv})
		ch <- runResult{last, err}
	}()
	return ch
}

func TestLoopResize(t *testing.T) {
	W := strings.Repeat("abcdefghij", 6)
	toks := []tokenize.Token{{Text: "wonderful"}, {Text: "wonderful"}, {Text: "wonderful"}, {Text: W}, {Text: "end"}}

	t.Run("relayout", func(t *testing.T) {
		p := state.NewPlayer(state.Config{Tokens: toks, WPM: 50, Size: 3})
		rec := hookResize(t, p)
		rt := newResizeTerm(80, 24)
		rt.sendKeys(" ")
		ch := startResizeRun(context.Background(), rt, p)

		n := 0
		var fr frameRec
		fr, n = waitSized(t, rec, n, 80, 24, func(m Model) bool { return m.Paused })
		if fr.f.Size != 3 || focusCol(fr.f) != 40 {
			t.Fatalf("80x24: Size %d, focus %d; want 3, 40", fr.f.Size, focusCol(fr.f))
		}
		rt.resize(40, 8)
		fr, n = waitSized(t, rec, n, 40, 8, nil)
		if fr.f.Size != 2 || focusCol(fr.f) != 20 {
			t.Fatalf("40x8: Size %d, focus %d; want 2, 20", fr.f.Size, focusCol(fr.f))
		}
		rt.resize(19, 4)
		fr, n = waitSized(t, rec, n, 19, 4, nil)
		if fr.f.Size != 0 || frameText(fr.f) != TooSmallText {
			t.Fatalf("19x4: Size %d, text %q; want 0, %q", fr.f.Size, frameText(fr.f), TooSmallText)
		}
		rt.resize(80, 24)
		fr, n = waitSized(t, rec, n, 80, 24, nil)
		if fr.f.Size != 3 || focusCol(fr.f) != 40 {
			t.Fatalf("80x24 again: Size %d, focus %d; want 3, 40", fr.f.Size, focusCol(fr.f))
		}

		rt.sendKeys("\x1b[C\x1b[C\x1b[C")
		fr, n = waitSized(t, rec, n, 80, 24, func(m Model) bool { return m.Index == 3 })
		if want := len(Split(W, EffectiveSize(W, 3, 80, 24), 80)); fr.f.Parts != want {
			t.Fatalf("W at 80x24: Parts %d, want %d", fr.f.Parts, want)
		}
		rt.resize(20, 24)
		fr, n = waitSized(t, rec, n, 20, 24, nil)
		if want := len(Split(W, EffectiveSize(W, 3, 20, 24), 20)); fr.f.Parts != want {
			t.Fatalf("W at 20x24: Parts %d, want %d", fr.f.Parts, want)
		}

		rng := rand.New(rand.NewPCG(49, 7))
		for i := 0; i < 200; i++ {
			rt.resize(rng.IntN(122)-1, rng.IntN(42)-1)
			if i%10 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
		rt.resize(80, 24)
		waitSized(t, rec, n, 80, 24, nil)
		select {
		case r := <-ch:
			t.Fatalf("Run ended during the resize burst: (%d, %v)", r.last, r.err)
		default:
		}
		if errs := rec.errors(); len(errs) > 0 {
			t.Fatalf("invariant violations (%d): %s", len(errs), strings.Join(errs[:min(5, len(errs))], "; "))
		}

		rt.sendKeys("q")
		r := waitRun(t, ch, 5*time.Second)
		if r.last != 3 || r.err != nil {
			t.Fatalf("Run = (%d, %v), want (3, nil)", r.last, r.err)
		}
		if out := rt.output(); !strings.HasSuffix(out, RestoreSeq) {
			t.Fatalf("output does not end with RestoreSeq: %q", out[max(0, len(out)-40):])
		}
	})

	t.Run("size error", func(t *testing.T) {
		p := state.NewPlayer(state.Config{Tokens: toks, WPM: 50, Size: 3})
		rec := hookResize(t, p)
		rt := newResizeTerm(80, 24)
		rt.sendKeys(" ")
		ch := startResizeRun(context.Background(), rt, p)
		waitSized(t, rec, 0, 80, 24, func(m Model) bool { return m.Paused })
		rt.smu.Lock()
		rt.sizeErr = errors.New("size boom")
		rt.smu.Unlock()
		rt.signal()
		r := waitRun(t, ch, 5*time.Second)
		if r.err == nil {
			t.Fatal("Run returned nil error after a Size error")
		}
		if out := rt.output(); !strings.HasSuffix(out, RestoreSeq) {
			t.Fatalf("output does not end with RestoreSeq")
		}
		if _, rs, cl := rt.counts(); rs < 1 || cl < 1 {
			t.Fatalf("Restore %d, Close %d; want both >= 1", rs, cl)
		}
	})
}
