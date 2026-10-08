package tui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/0x6c6d/fastread/internal/state"
)

// Test seams; production code never changes them.
var (
	renderFn  = Render
	encodeFn  = Encode
	frameHook func(m Model, f Frame) // nil in production; called after each written frame
)

// modelOf builds the render model for the player's current state.
func modelOf(p *state.Player) Model {
	ew, ok := p.EffectiveWPM()
	if !ok {
		ew = 0
	}
	return Model{
		Word:         p.Token().Text,
		Size:         p.Size(),
		Paused:       !p.Playing(),
		ShowProgress: p.ShowProgress(),
		ShowHelp:     p.ShowHelp(),
		Index:        p.Index(),
		Total:        p.Len(),
		WPM:          p.WPM(),
		EffectiveWPM: ew,
		Part:         p.Part(),
	}
}

// Run plays p on t until a quit key, the end of the text or ctx is done, and returns the
// index of the word on screen. The caller checks p.Finished() to tell end-of-text from quit.
// All timing comes from p.Deadline(); the loop never schedules "now + delay" itself.
//
// Exit paths: a quit key, ctx done and the end of the text return (p.Index(), nil); a key
// read error, a frame write error, a size error or a panic anywhere in the loop return
// p.Index() and a "tui: ..." error. After a successful MakeRaw, cleanup runs exactly once
// on every path: RestoreSeq (always the last write), Restore, Close, each guarded so a
// failure or panic in one does not skip the rest; Run returns only after the reader
// goroutine (the only goroutine it starts) has exited and both timers are stopped.
func Run(ctx context.Context, t Terminal, p *state.Player, opts Options) (last int, err error) {
	if err := t.MakeRaw(); err != nil {
		guard(t.Close)
		return safeIndex(p), fmt.Errorf("tui: raw mode: %w", err)
	}

	done := make(chan struct{})       // closed by cleanup before Close
	readerDone := make(chan struct{}) // closed when the reader goroutine exits
	readerStarted := false
	// Both timers start stopped; a nil channel means "not armed".
	deadline := time.NewTimer(time.Hour)
	deadline.Stop()
	escTimer := time.NewTimer(time.Hour)
	escTimer.Stop()

	defer func() {
		if r := recover(); r != nil {
			last = safeIndex(p)
			err = fmt.Errorf("tui: internal error: %v", r)
		}
		deadline.Stop()
		escTimer.Stop()
		guard(func() error { _, e := t.Write([]byte(RestoreSeq)); return e })
		guard(t.Restore)
		close(done)
		guard(t.Close)
		if readerStarted {
			<-readerDone
		}
	}()

	if _, err := t.Write([]byte(EnterSeq)); err != nil {
		return p.Index(), fmt.Errorf("tui: writing frame: %w", err)
	}

	keys := make(chan []byte)
	readErr := make(chan error) // unbuffered: the reader selects on done as well
	readerStarted = true
	go func() {
		defer close(readerDone)
		var rerr error
		defer func() {
			if r := recover(); r != nil {
				rerr = fmt.Errorf("tui: internal error: %v", r)
			}
			if rerr != nil {
				select {
				case readErr <- rerr:
				case <-done:
				}
			}
		}()
		buf := make([]byte, 256)
		for {
			n, err := t.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				select {
				case keys <- chunk:
				case <-done:
					return
				}
			}
			if err != nil {
				// An error caused by cleanup's Close is never reported: done is
				// closed before Close and the loop no longer receives.
				rerr = fmt.Errorf("tui: reading keys: %w", err)
				return
			}
		}
	}()

	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	mode := DetectColorMode(getenv)

	redraw := func() error {
		w, h, err := t.Size()
		if err != nil {
			return fmt.Errorf("tui: terminal size: %w", err)
		}
		word := p.Token().Text
		if s := EffectiveSize(word, p.Size(), w, h); s == 0 {
			p.SetParts(nil)
		} else {
			p.SetParts(Split(word, s, w))
		}
		m := modelOf(p)
		f := renderFn(m, w, h)
		if _, err := t.Write(encodeFn(f, mode)); err != nil {
			return fmt.Errorf("tui: writing frame: %w", err)
		}
		if frameHook != nil {
			frameHook(m, f)
		}
		return nil
	}

	var deadlineC, escC <-chan time.Time

	armDeadline := func() {
		if p.Playing() {
			deadline.Reset(time.Until(p.Deadline()))
			deadlineC = deadline.C
		} else {
			deadline.Stop()
			deadlineC = nil
		}
	}
	armEsc := func(pending bool) {
		if pending {
			escTimer.Reset(EscTimeout)
			escC = escTimer.C
		} else {
			escTimer.Stop()
			escC = nil
		}
	}

	var dec KeyDecoder
	// apply performs actions in order; it reports whether one of them is a quit.
	apply := func(acts []state.Action) bool {
		for _, a := range acts {
			if a == state.ActQuit {
				return true
			}
			p.Apply(a)
		}
		return false
	}

	// A terminal that reports size changes adds one more wake-up; nil otherwise.
	var resizeC <-chan struct{}
	if rz, ok := t.(Resizer); ok {
		resizeC = rz.Resized()
	}

	p.Start()
	if err := redraw(); err != nil {
		return p.Index(), err
	}
	armDeadline()
	for {
		select {
		case <-ctx.Done():
			return p.Index(), nil
		case b := <-keys:
			if apply(dec.Feed(b)) {
				return p.Index(), nil
			}
			armEsc(dec.Pending())
		case err := <-readErr:
			return p.Index(), err
		case <-escC:
			escC = nil
			if apply(dec.Flush()) {
				return p.Index(), nil
			}
		case <-deadlineC:
			deadlineC = nil
			if p.Tick() {
				return p.Index(), nil
			}
		case <-resizeC:
			// redraw below re-queries the size and re-splits; SetParts keeps the
			// current step's elapsed time.
		}
		if err := redraw(); err != nil {
			return p.Index(), err
		}
		armDeadline()
	}
}

// guard runs one cleanup step; its error and any panic are dropped so the next step runs.
func guard(f func() error) {
	defer func() { _ = recover() }()
	_ = f()
}

// safeIndex is p.Index(), or 0 if that panics (a broken player must not break cleanup).
func safeIndex(p *state.Player) (i int) {
	defer func() {
		if recover() != nil {
			i = 0
		}
	}()
	return p.Index()
}
