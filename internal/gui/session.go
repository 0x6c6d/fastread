package gui

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/0x6c6d/fastread/internal/state"
)

// session ends a GUI run exactly once: finish(p.Index(), err) is called by the first
// end; later calls (from any goroutine) do nothing.
type session struct {
	p      *state.Player
	finish func(last int, err error)
	once   sync.Once
	isDone atomic.Bool
	done   chan struct{} // closed by the first end, before finish is called
}

func newSession(p *state.Player, finish func(last int, err error)) *session {
	return &session{p: p, finish: finish, done: make(chan struct{})}
}

// end reports the outcome of the run. Only the first call has an effect: it marks the
// session done, closes the done channel and calls finish(p.Index(), err). A panic in
// finish propagates to the caller (guard contains it); the session stays done.
func (s *session) end(err error) {
	s.once.Do(func() {
		s.isDone.Store(true)
		close(s.done)
		if s.finish != nil {
			s.finish(s.p.Index(), err)
		}
	})
}

// ended reports whether end has been called; it is safe for concurrent use.
func (s *session) ended() bool { return s.isDone.Load() }

// doneCh is closed by the first end.
func (s *session) doneCh() <-chan struct{} { return s.done }

// guard is deferred at the top of the window goroutine: a panic becomes
// end(fmt.Errorf("gui: internal error: %v", r)); it never re-panics.
func (s *session) guard() {
	r := recover()
	if r == nil {
		return
	}
	defer func() { _ = recover() }() // finish panicking again must not escape either
	s.end(fmt.Errorf("gui: internal error: %s", oneLine(fmt.Sprint(r))))
}

// windowErr maps app.DestroyEvent.Err: nil → nil (close button = quit); else
// fmt.Errorf("gui: window: %w", err), newlines replaced by spaces.
func windowErr(err error) error {
	if err == nil {
		return nil
	}
	return &lineErr{msg: "gui: window: " + oneLine(err.Error()), err: err}
}

// lineErr is a single-line message that still unwraps to the original error.
type lineErr struct {
	msg string
	err error
}

func (e *lineErr) Error() string { return e.msg }
func (e *lineErr) Unwrap() error { return e.err }

// oneLine replaces line breaks with spaces and trims surrounding blanks.
func oneLine(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s))
}
