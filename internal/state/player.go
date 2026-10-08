// Package state holds the playback model and resume storage. It must never import UI packages.
package state

import (
	"math"
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
	remaining    time.Duration // time left on the current word while paused
	clock        Clock

	started   bool
	completed int           // words whose deadline passed inside Tick
	unpaused  time.Duration // closed playing segments
	segStart  time.Time     // start of the open playing segment
	endTime   time.Time     // when the player finished
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
	p.started = true
	p.completed = 0
	p.unpaused = 0
	p.segStart = p.clock.Now()
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
		p.completed++
		if p.index >= len(p.tokens)-1 {
			p.finished = true
			p.endTime = now
			p.unpaused += now.Sub(p.segStart)
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
	if p.finished || len(p.tokens) == 0 {
		return false
	}
	now := p.clock.Now()
	switch a {
	case ActTogglePause:
		if p.playing {
			p.remaining = p.deadline.Sub(now)
			if p.remaining < 0 {
				p.remaining = 0
			}
			p.playing = false
			p.unpaused += now.Sub(p.segStart)
		} else {
			p.segStart = now
			p.deadline = now.Add(p.remaining)
			p.playing = true
		}
	case ActWPMUp:
		p.wpm = clampInt(p.wpm+25, timing.MinWPM, timing.MaxWPM)
	case ActWPMDown:
		p.wpm = clampInt(p.wpm-25, timing.MinWPM, timing.MaxWPM)
	case ActSizeUp:
		p.size = clampInt(p.size+1, 1, 5)
	case ActSizeDown:
		p.size = clampInt(p.size-1, 1, 5)
	case ActNext, ActPrev:
		step := 1
		if p.playing {
			step = 10
		}
		if a == ActPrev {
			step = -step
		}
		p.jump(clampInt(p.index+step, 0, len(p.tokens)-1), now)
	case ActHome:
		p.jump(0, now)
	case ActToggleProgress:
		p.showProgress = !p.showProgress
	case ActToggleHelp:
		p.showHelp = !p.showHelp
	}
	return false
}

// jump moves to index i and restarts the word timer if the index changed.
func (p *Player) jump(i int, now time.Time) {
	if i == p.index {
		return
	}
	p.index = i
	if p.playing {
		p.deadline = now.Add(p.delay())
	} else {
		p.remaining = p.delay()
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Deadline returns when the current word should be replaced. While paused it
// returns the deadline that resuming at the current Now() would give (now + remaining).
func (p *Player) Deadline() time.Time {
	if !p.playing {
		return p.clock.Now().Add(p.remaining)
	}
	return p.deadline
}

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

// EffectiveWPM returns words completed per minute of unpaused playing time, rounded to the
// nearest integer. ok is false before Start, while fewer than 2 words were completed, or
// when no unpaused time has elapsed (the UI then shows "—").
func (p *Player) EffectiveWPM() (wpm int, ok bool) {
	if !p.started || p.completed < 2 {
		return 0, false
	}
	t := p.unpaused
	if p.playing && !p.finished {
		t += p.clock.Now().Sub(p.segStart)
	}
	if t <= 0 {
		return 0, false
	}
	return int(math.Round(float64(p.completed) / t.Minutes())), true
}
