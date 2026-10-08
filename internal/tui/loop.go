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
func Run(ctx context.Context, t Terminal, p *state.Player, opts Options) (last int, err error) {
	if err := t.MakeRaw(); err != nil {
		t.Close()
		return p.Index(), fmt.Errorf("tui: raw mode: %w", err)
	}

	done := make(chan struct{})
	readerDone := make(chan struct{})
	readerStarted := false
	defer func() {
		r := recover()
		t.Write([]byte(RestoreSeq))
		t.Restore()
		close(done)
		t.Close()
		if readerStarted {
			<-readerDone
		}
		if r != nil {
			last = p.Index()
			err = fmt.Errorf("tui: internal error: %v", r)
		}
	}()

	if _, err := t.Write([]byte(EnterSeq)); err != nil {
		return p.Index(), fmt.Errorf("tui: write: %w", err)
	}

	keys := make(chan []byte)
	readerStarted = true
	go func() {
		defer close(readerDone)
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
			return fmt.Errorf("tui: write: %w", err)
		}
		if frameHook != nil {
			frameHook(m, f)
		}
		return nil
	}

	// Both timers start stopped; a nil channel means "not armed".
	deadline := time.NewTimer(time.Hour)
	deadline.Stop()
	defer deadline.Stop()
	escTimer := time.NewTimer(time.Hour)
	escTimer.Stop()
	defer escTimer.Stop()
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
