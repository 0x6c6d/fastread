// Package state holds the playback model and resume storage. It must never import UI packages.
package state

import (
	"time"

	"github.com/0x6c6d/fastread/internal/timing"
	"github.com/0x6c6d/fastread/internal/tokenize"
)

// Action is a user command the UI maps keys to.
type Action int

// Actions the player understands.
const (
	ActNone           Action = iota
	ActTogglePause           // Space
	ActWPMUp                 // Up: +25
	ActWPMDown               // Down: -25
	ActSizeDown              // [
	ActSizeUp                // ]
	ActPrev                  // Left
	ActNext                  // Right
	ActHome                  // Home
	ActToggleProgress        // p
	ActToggleHelp            // ?
	ActQuit                  // q, Esc, Ctrl+C
)

// Config configures a Player.
type Config struct {
	Tokens       []tokenize.Token // non-empty
	Start        int              // first word index, already validated by the caller
	WPM          int              // 50..1500
	Size         int              // 1..5
	ShowProgress bool
	Clock        Clock // nil means SystemClock{}
}

// Player is the playback model both UIs drive. It performs no I/O and starts no
// goroutines or timers: the UI asks for Deadline and calls Tick.
type Player struct {
	tokens       []tokenize.Token
	index        int
	wpm          int
	size         int
	showProgress bool
	showHelp     bool
	playing      bool
	finished     bool
	deadline     time.Time
	clock        Clock
}

// NewPlayer returns a Player in the playing state. Start is clamped to [0, len-1].
func NewPlayer(cfg Config) *Player {
	clk := cfg.Clock
	if clk == nil {
		clk = SystemClock{}
	}
	idx := cfg.Start
	if idx > len(cfg.Tokens)-1 {
		idx = len(cfg.Tokens) - 1
	}
	if idx < 0 {
		idx = 0
	}
	p := &Player{
		tokens:       cfg.Tokens,
		index:        idx,
		wpm:          cfg.WPM,
		size:         cfg.Size,
		showProgress: cfg.ShowProgress,
		playing:      true,
		clock:        clk,
	}
	p.Start()
	return p
}

// delay returns how long the current word is shown.
func (p *Player) delay() time.Duration {
	if len(p.tokens) == 0 {
		return 0
	}
	t := p.tokens[p.index]
	return timing.DelayPara(t.Text, p.wpm, t.ParaEnd)
}

// Start sets the deadline to now plus the delay of the current word.
func (p *Player) Start() {
	p.deadline = p.clock.Now().Add(p.delay())
}

// Tick advances past every word whose deadline has passed. It reports whether the
// end of the text has been reached.
func (p *Player) Tick() (finished bool) {
	if p.finished {
		return true
	}
	now := p.clock.Now()
	for p.playing && !now.Before(p.deadline) {
		if p.index >= len(p.tokens)-1 {
			p.finished = true
			return true
		}
		p.index++
		p.deadline = p.deadline.Add(p.delay())
	}
	return false
}

// Apply performs a user action and reports whether the program should quit.
func (p *Player) Apply(a Action) (quit bool) {
	if a == ActQuit {
		return true
	}
	// TODO(phase3): R26 keys
	return false
}

// Deadline returns when the current word should be replaced.
func (p *Player) Deadline() time.Time { return p.deadline }

// Index returns the index of the word on screen.
func (p *Player) Index() int { return p.index }

// Len returns the number of words.
func (p *Player) Len() int { return len(p.tokens) }

// Token returns the current word.
func (p *Player) Token() tokenize.Token {
	if len(p.tokens) == 0 {
		return tokenize.Token{}
	}
	return p.tokens[p.index]
}

// Playing reports whether playback is running.
func (p *Player) Playing() bool { return p.playing }

// Finished reports whether Tick has reported the end of the text.
func (p *Player) Finished() bool { return p.finished }

// WPM returns the current reading speed.
func (p *Player) WPM() int { return p.wpm }

// Size returns the current display size.
func (p *Player) Size() int { return p.size }

// ShowProgress reports whether the progress indicator is shown.
func (p *Player) ShowProgress() bool { return p.showProgress }

// ShowHelp reports whether the help overlay is shown.
func (p *Player) ShowHelp() bool { return p.showHelp }
