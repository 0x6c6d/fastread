package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/0x6c6d/fastread/internal/state"
)

// Options configures Run.
type Options struct {
	Getenv func(string) string // colour detection; nil -> os.Getenv
}

// modelOf builds the render model for the player's current state.
func modelOf(p *state.Player) Model {
	return Model{
		Word:         p.Token().Text,
		Size:         p.Size(),
		Paused:       !p.Playing(),
		ShowProgress: p.ShowProgress(),
		ShowHelp:     p.ShowHelp(),
		Index:        p.Index(),
		Total:        p.Len(),
		WPM:          p.WPM(),
	}
}

// isQuitChunk reports whether a chunk of key bytes asks to quit: it contains 'q' or
// Ctrl+C, or it is exactly a lone Esc.
func isQuitChunk(b []byte) bool {
	if bytes.IndexByte(b, 'q') >= 0 || bytes.IndexByte(b, 0x03) >= 0 {
		return true
	}
	return len(b) == 1 && b[0] == 0x1b
}

// Run plays p on t until a quit key, the end of the text or ctx is done, and returns the
// index of the word on screen. The caller checks p.Finished() to tell end-of-text from quit.
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
	draw := func() error {
		w, h, err := t.Size()
		if err != nil {
			return fmt.Errorf("tui: terminal size: %w", err)
		}
		if _, err := t.Write(Encode(Render(modelOf(p), w, h), mode)); err != nil {
			return fmt.Errorf("tui: write: %w", err)
		}
		return nil
	}

	p.Start()
	if err := draw(); err != nil {
		return p.Index(), err
	}
	timer := time.NewTimer(time.Until(p.Deadline()))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return p.Index(), nil
		case b := <-keys:
			if isQuitChunk(b) {
				p.Apply(state.ActQuit)
				return p.Index(), nil
			}
		case <-timer.C:
			if p.Tick() {
				return p.Index(), nil
			}
			if err := draw(); err != nil {
				return p.Index(), err
			}
			timer.Reset(time.Until(p.Deadline()))
		}
	}
}
