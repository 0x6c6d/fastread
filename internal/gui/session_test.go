package gui

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/0x6c6d/fastread/internal/state"
	"github.com/0x6c6d/fastread/internal/tokenize"
)

// finishCall is one recorded call of the fake finish.
type finishCall struct {
	last int
	err  error
}

// recorder is a fake finish that records every call; panicMsg != "" makes it panic
// after recording.
type recorder struct {
	mu       sync.Mutex
	calls    []finishCall
	panicMsg string
}

func (r *recorder) finish(last int, err error) {
	r.mu.Lock()
	r.calls = append(r.calls, finishCall{last, err})
	r.mu.Unlock()
	if r.panicMsg != "" {
		panic(r.panicMsg)
	}
}

func sessionPlayer(t *testing.T) *state.Player {
	t.Helper()
	var toks []tokenize.Token
	for i := 0; i < 30; i++ {
		toks = append(toks, tokenize.Token{Text: "word"})
	}
	return state.NewPlayer(state.Config{Tokens: toks, WPM: 300, Size: 3})
}

// guarded runs f with s.guard deferred and reports whether anything escaped.
func guarded(s *session, f func()) (escaped any) {
	defer func() { escaped = recover() }()
	func() {
		defer s.guard()
		f()
	}()
	return nil
}

func TestGUIExitPath(t *testing.T) {
	errA := errors.New("error A")
	tests := []struct {
		name     string
		panicMsg string                                          // fake finish panics with this
		run      func(t *testing.T, s *session, p *state.Player) // drives the session
		wantErr  func(t *testing.T, err error)                   // checks the single error
	}{
		{
			name: "end nil",
			run:  func(t *testing.T, s *session, p *state.Player) { s.end(nil) },
			wantErr: func(t *testing.T, err error) {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			},
		},
		{
			name: "end after ActNext passes the new index",
			run: func(t *testing.T, s *session, p *state.Player) {
				p.Apply(state.ActNext)
				if p.Index() == 0 {
					t.Fatal("ActNext did not move the index")
				}
				s.end(nil)
			},
			wantErr: func(t *testing.T, err error) {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			},
		},
		{
			name: "first end wins",
			run:  func(t *testing.T, s *session, p *state.Player) { s.end(errA); s.end(nil); s.end(errors.New("later")) },
			wantErr: func(t *testing.T, err error) {
				if err != errA {
					t.Errorf("err = %v, want errA", err)
				}
			},
		},
		{
			name: "50 concurrent ends",
			run: func(t *testing.T, s *session, p *state.Player) {
				var wg sync.WaitGroup
				start := make(chan struct{})
				for i := 0; i < 50; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						<-start
						s.end(nil)
						_ = s.ended()
					}()
				}
				close(start)
				wg.Wait()
			},
			wantErr: func(t *testing.T, err error) {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			},
		},
		{
			name: "panic becomes internal error",
			run: func(t *testing.T, s *session, p *state.Player) {
				if e := guarded(s, func() { panic("boom") }); e != nil {
					t.Errorf("panic escaped guard: %v", e)
				}
			},
			wantErr: func(t *testing.T, err error) {
				if err == nil || err.Error() != "gui: internal error: boom" {
					t.Errorf("err = %v, want gui: internal error: boom", err)
				}
			},
		},
		{
			name: "multi-line panic stays one line",
			run: func(t *testing.T, s *session, p *state.Player) {
				if e := guarded(s, func() { panic("bo\nom") }); e != nil {
					t.Errorf("panic escaped guard: %v", e)
				}
			},
			wantErr: func(t *testing.T, err error) {
				if err == nil || strings.Contains(err.Error(), "\n") ||
					!strings.HasPrefix(err.Error(), "gui: internal error: ") {
					t.Errorf("err = %q, want one line gui: internal error: ...", err)
				}
			},
		},
		{
			name: "panic after end",
			run: func(t *testing.T, s *session, p *state.Player) {
				if e := guarded(s, func() { s.end(nil); panic("late") }); e != nil {
					t.Errorf("panic escaped guard: %v", e)
				}
			},
			wantErr: func(t *testing.T, err error) {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			},
		},
		{
			name:     "finish panics",
			panicMsg: "finish exploded",
			run: func(t *testing.T, s *session, p *state.Player) {
				if e := guarded(s, func() { s.end(nil) }); e != nil {
					t.Errorf("panic escaped guard: %v", e)
				}
				s.end(errA) // still done: no second call
			},
			wantErr: func(t *testing.T, err error) {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			},
		},
		{
			name:     "finish panics after a guarded panic",
			panicMsg: "finish exploded",
			run: func(t *testing.T, s *session, p *state.Player) {
				if e := guarded(s, func() { panic("boom") }); e != nil {
					t.Errorf("panic escaped guard: %v", e)
				}
			},
			wantErr: func(t *testing.T, err error) {
				if err == nil || err.Error() != "gui: internal error: boom" {
					t.Errorf("err = %v, want gui: internal error: boom", err)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := sessionPlayer(t)
			r := &recorder{panicMsg: tc.panicMsg}
			s := newSession(p, r.finish)
			if s.ended() {
				t.Fatal("ended() before end")
			}
			tc.run(t, s, p)
			if !s.ended() {
				t.Fatal("ended() = false after end")
			}
			select {
			case <-s.doneCh():
			default:
				t.Error("done channel not closed after end")
			}
			r.mu.Lock()
			calls := r.calls
			r.mu.Unlock()
			if len(calls) != 1 {
				t.Fatalf("finish called %d times, want 1: %v", len(calls), calls)
			}
			if want := p.Index(); calls[0].last != want {
				t.Errorf("last = %d, want p.Index() = %d", calls[0].last, want)
			}
			tc.wantErr(t, calls[0].err)
		})
	}

	t.Run("windowErr", func(t *testing.T) {
		if err := windowErr(nil); err != nil {
			t.Errorf("windowErr(nil) = %v, want nil", err)
		}
		for _, e := range []error{
			errors.New("wayland: wl_display_connect failed: no such file"),
			errors.New("x11: cannot open display\nwayland: wl_display_connect failed\n"),
		} {
			got := windowErr(e)
			if !errors.Is(got, e) {
				t.Errorf("windowErr(%q) does not wrap the original", e)
			}
			if !strings.HasPrefix(got.Error(), "gui: window: ") {
				t.Errorf("windowErr(%q) = %q, want prefix gui: window: ", e, got)
			}
			if strings.Contains(got.Error(), "\n") {
				t.Errorf("windowErr(%q) = %q contains a newline", e, got)
			}
		}
	})
}
