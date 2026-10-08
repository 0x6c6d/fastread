package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/state"
	"github.com/0x6c6d/fastread/internal/timing"
	"github.com/0x6c6d/fastread/internal/tokenize"
)

// frameRec is one frame seen through frameHook.
type frameRec struct {
	m    Model
	f    Frame
	at   time.Time
	word string // the word row's text (size 1 only meaningful)
}

// recorder collects frames written by Run.
type recorder struct {
	mu     sync.Mutex
	frames []frameRec
	errs   []string
}

func (r *recorder) all() []frameRec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]frameRec(nil), r.frames...)
}

func (r *recorder) n() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.frames)
}

func (r *recorder) last() frameRec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.frames[len(r.frames)-1]
}

// wordRow returns the text of the size-1 word row of f.
func wordRow(f Frame) string {
	if f.H == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range f.Cells[f.H/2] {
		b.WriteString(c.Text)
	}
	return strings.TrimSpace(b.String())
}

// hookFrames installs a frameHook that records frames and checks the per-frame
// invariants against p; the hook is removed at test cleanup.
func hookFrames(t *testing.T, p *state.Player) *recorder {
	t.Helper()
	r := &recorder{}
	old := frameHook
	t.Cleanup(func() { frameHook = old })
	frameHook = func(m Model, f Frame) {
		rec := frameRec{m: m, f: f, at: time.Now(), word: wordRow(f)}
		var e string
		if f.Parts > 0 && (m.Part >= f.Parts || p.Parts() != f.Parts) {
			e = fmt.Sprintf("frame (%d,%d): Part %d, f.Parts %d, p.Parts() %d", m.Index, m.Part, m.Part, f.Parts, p.Parts())
		}
		r.mu.Lock()
		r.frames = append(r.frames, rec)
		if e != "" {
			r.errs = append(r.errs, e)
		}
		r.mu.Unlock()
	}
	return r
}

// startRun runs Run in a goroutine; the result arrives on the returned channel.
func startRun(ctx context.Context, ft *fakeTerm, p *state.Player) <-chan runResult {
	ch := make(chan runResult, 1)
	go func() {
		last, err := Run(ctx, ft, p, Options{Getenv: noEnv})
		ch <- runResult{last, err}
	}()
	return ch
}

// waitRun waits up to d for Run to return.
func waitRun(t *testing.T, ch <-chan runResult, d time.Duration) runResult {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(d):
		t.Fatalf("Run did not return within %v", d)
		return runResult{}
	}
}

// waitFrame polls until a frame with index >= from satisfies ok, or fails after 2 s.
func waitFrame(t *testing.T, r *recorder, from int, what string, ok func(Model) bool) frameRec {
	t.Helper()
	return waitFrameFor(t, r, from, 2*time.Second, what, ok)
}

// waitFrameFor is waitFrame with timeout d.
func waitFrameFor(t *testing.T, r *recorder, from int, d time.Duration, what string, ok func(Model) bool) frameRec {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		fs := r.all()
		for i := from; i < len(fs); i++ {
			if ok(fs[i].m) {
				return fs[i]
			}
		}
		if time.Now().After(deadline) {
			last := "none"
			if len(fs) > 0 {
				last = fmt.Sprintf("%+v", fs[len(fs)-1].m)
			}
			t.Fatalf("no frame with %s; last model %s", what, last)
		}
		time.Sleep(time.Millisecond)
	}
}

func words(n int, f func(i int) string) []tokenize.Token {
	toks := make([]tokenize.Token, n)
	for i := range toks {
		toks[i] = tokenize.Token{Text: f(i)}
	}
	return toks
}

func TestLoopKeys(t *testing.T) {
	steps := []struct {
		key  string
		what string
		ok   func(Model) bool
	}{
		{" ", "Paused", func(m Model) bool { return m.Paused }},
		{"\x1b[C", "Index 1", func(m Model) bool { return m.Index == 1 }},
		{"\x1bOC", "Index 2", func(m Model) bool { return m.Index == 2 }},
		{"\x1b[D", "Index 1", func(m Model) bool { return m.Index == 1 }},
		{"\x1b[1~", "Index 0", func(m Model) bool { return m.Index == 0 }},
		{"]", "Size 3", func(m Model) bool { return m.Size == 3 }},
		{"[", "Size 2", func(m Model) bool { return m.Size == 2 }},
		// The brief says 325/300, which contradicts its own 50 wpm start (Up = +25).
		{"\x1b[A", "WPM 75", func(m Model) bool { return m.WPM == 75 }},
		{"\x1b[B", "WPM 50", func(m Model) bool { return m.WPM == 50 }},
		{"p", "ShowProgress false", func(m Model) bool { return !m.ShowProgress }},
		{"?", "ShowHelp true", func(m Model) bool { return m.ShowHelp }},
		{" ", "playing", func(m Model) bool { return !m.Paused }},
		{"\x1b[C", "Index 10", func(m Model) bool { return m.Index == 10 }},
	}
	for _, tc := range []struct{ name, key string }{
		{"q", "q"},
		{"ctrl-c", "\x03"},
		{"esc", "\x1b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ft := newFakeTerm()
			ft.w0, ft.h0 = 80, 24
			toks := words(30, func(i int) string { return fmt.Sprintf("w%d", i) })
			p := state.NewPlayer(state.Config{Tokens: toks, WPM: 50, Size: 2, ShowProgress: true})
			r := hookFrames(t, p)
			ch := startRun(context.Background(), ft, p)
			t.Cleanup(func() { ft.Close() })
			waitFrame(t, r, 0, "first frame", func(Model) bool { return true })
			send := func(s string) {
				if _, err := ft.w.Write([]byte(s)); err != nil {
					t.Fatalf("send %q: %v", s, err)
				}
			}
			for _, st := range steps {
				mark := r.n()
				send(st.key)
				waitFrame(t, r, mark, st.what+" after "+fmt.Sprintf("%q", st.key), st.ok)
			}

			// Esc and the rest of the sequence in separate chunks: one Right, no quit.
			prev := r.last().m.Index
			mark := r.n()
			send("\x1b")
			time.Sleep(10 * time.Millisecond)
			send("[C")
			got := waitFrame(t, r, mark, "Index >= prev+10", func(m Model) bool { return m.Index >= prev+10 })
			if got.m.Index != 20 && got.m.Index != prev+10 {
				t.Fatalf("split Esc+[C: Index %d, want 20", got.m.Index)
			}
			select {
			case res := <-ch:
				t.Fatalf("Run returned after split Esc+[C: %+v", res)
			default:
			}

			// Garbage changes nothing and does not quit.
			before := r.last().m
			for _, g := range []string{"\x00\xffQx\x1b[5~\x1bOP", "\x1b[" + strings.Repeat("9", 100) + "~"} {
				mark := r.n()
				send(g)
				waitFrame(t, r, mark, "frame after garbage", func(Model) bool { return true })
			}
			time.Sleep(2 * EscTimeout)
			after := r.last().m
			if after.Paused != before.Paused || after.Size != before.Size || after.WPM != before.WPM ||
				after.ShowProgress != before.ShowProgress || after.ShowHelp != before.ShowHelp ||
				after.Index != before.Index {
				t.Fatalf("garbage changed the model: before %+v, after %+v", before, after)
			}
			select {
			case res := <-ch:
				t.Fatalf("Run returned after garbage: %+v", res)
			default:
			}

			start := time.Now()
			send(tc.key)
			res := waitRun(t, ch, time.Second)
			if el := time.Since(start); tc.key == "\x1b" && el < EscTimeout {
				t.Errorf("lone Esc quit after %v, want >= EscTimeout", el)
			}
			if want := r.last().m.Index; res.last != want || res.err != nil {
				t.Fatalf("Run = (%d, %v), want (%d, nil)", res.last, res.err, want)
			}
			if !strings.HasSuffix(ft.output(), RestoreSeq) {
				t.Error("output does not end with RestoreSeq")
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			if len(r.errs) > 0 {
				t.Errorf("frame invariants: %v", r.errs)
			}
		})
	}
}

type ip struct{ index, part int }

// dedup returns the (Index, Part) sequence of fs without consecutive duplicates, with the
// first frame of each pair.
func dedup(fs []frameRec) ([]ip, []frameRec) {
	var seq []ip
	var first []frameRec
	for _, f := range fs {
		k := ip{f.m.Index, f.m.Part}
		if len(seq) > 0 && seq[len(seq)-1] == k {
			continue
		}
		seq = append(seq, k)
		first = append(first, f)
	}
	return seq, first
}

func TestLoopSplitSteps(t *testing.T) {
	W := strings.Repeat("abcdefghij", 6)
	parts := Split(W, 1, 20)
	if len(parts) != 5 {
		t.Fatalf("Split(W, 1, 20) has %d parts, want 5: %q", len(parts), parts)
	}
	wantSeq := []ip{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {1, 3}, {1, 4}, {2, 0}}
	newP := func(wpm int) *state.Player {
		toks := []tokenize.Token{{Text: "go"}, {Text: W}, {Text: "end."}}
		return state.NewPlayer(state.Config{Tokens: toks, WPM: wpm, Size: 1})
	}
	newT := func() *fakeTerm {
		ft := newFakeTerm()
		ft.w0, ft.h0 = 20, 24
		return ft
	}
	checkFrames := func(t *testing.T, r *recorder, fs []frameRec) {
		t.Helper()
		r.mu.Lock()
		errs := r.errs
		r.mu.Unlock()
		if len(errs) > 0 {
			t.Errorf("frame invariants: %v", errs)
		}
		for _, f := range fs {
			if f.m.Index == 1 && f.word != parts[f.m.Part] {
				t.Errorf("frame (1,%d) word row %q, want %q", f.m.Part, f.word, parts[f.m.Part])
			}
		}
	}

	t.Run("1500wpm", func(t *testing.T) {
		ft := newT()
		p := newP(1500)
		r := hookFrames(t, p)
		res := waitRun(t, startRun(context.Background(), ft, p), 5*time.Second)
		if res.last != 2 || res.err != nil || !p.Finished() {
			t.Fatalf("Run = (%d, %v), finished %v; want (2, nil), true", res.last, res.err, p.Finished())
		}
		fs := r.all()
		seq, first := dedup(fs)
		if fmt.Sprint(seq) != fmt.Sprint(wantSeq) {
			t.Fatalf("(Index, Part) sequence %v, want %v", seq, wantSeq)
		}
		checkFrames(t, r, fs)
		var want time.Duration
		for i, s := range parts {
			if i < len(parts)-1 {
				want += timing.Delay(strings.TrimSuffix(s, "-"), 1500)
			} else {
				want += timing.DelayPara(s, 1500, false)
			}
		}
		got := first[6].at.Sub(first[1].at)
		if got < want*3/4 || got > want*5/4 {
			t.Errorf("(1,0)->(2,0) took %v, want %v ±25%%", got, want)
		}
	})

	t.Run("100wpm-pause", func(t *testing.T) {
		ft := newT()
		p := newP(100)
		r := hookFrames(t, p)
		ch := startRun(context.Background(), ft, p)
		t.Cleanup(func() { ft.Close() })
		waitFrameFor(t, r, 0, 10*time.Second, "(1,2)", func(m Model) bool { return m.Index == 1 && m.Part == 2 })
		mark := r.n()
		if _, err := ft.w.Write([]byte(" ")); err != nil {
			t.Fatal(err)
		}
		waitFrame(t, r, mark, "Paused", func(m Model) bool { return m.Paused })
		time.Sleep(300 * time.Millisecond)
		if m := r.last().m; m.Index != 1 || m.Part != 2 || !m.Paused {
			t.Fatalf("after 300 ms paused: %+v, want (1,2) paused", m)
		}
		if _, err := ft.w.Write([]byte(" ")); err != nil {
			t.Fatal(err)
		}
		res := waitRun(t, ch, 10*time.Second)
		if res.last != 2 || res.err != nil || !p.Finished() {
			t.Fatalf("Run = (%d, %v), finished %v; want (2, nil), true", res.last, res.err, p.Finished())
		}
		fs := r.all()
		seq, _ := dedup(fs)
		if fmt.Sprint(seq) != fmt.Sprint(wantSeq) {
			t.Fatalf("(Index, Part) sequence %v, want %v", seq, wantSeq)
		}
		checkFrames(t, r, fs)
	})
}

func TestLoopDriftRealClock(t *testing.T) {
	texts := []string{"word", "word,", "word.", "extraordinarily", "naïve"}
	toks := make([]tokenize.Token, 100)
	var want time.Duration
	for i := range toks {
		toks[i] = tokenize.Token{Text: texts[i%len(texts)], ParaEnd: i%10 == 9 || i == len(toks)-1}
		want += timing.DelayPara(toks[i].Text, 1500, toks[i].ParaEnd)
	}
	p := state.NewPlayer(state.Config{Tokens: toks, WPM: 1500, Size: 1, Clock: state.SystemClock{}})
	ft := newFakeTerm()
	ft.w0, ft.h0 = 80, 24
	ft.writeDelay = 5 * time.Millisecond
	start := time.Now()
	res := waitRun(t, startRun(context.Background(), ft, p), 3*want)
	got := time.Since(start)
	if res.last != 99 || res.err != nil || !p.Finished() {
		t.Fatalf("Run = (%d, %v), finished %v; want (99, nil), true", res.last, res.err, p.Finished())
	}
	if got < want*95/100 || got > want*105/100 {
		t.Errorf("Run took %v, want %v ±5%%", got, want)
	}
}
