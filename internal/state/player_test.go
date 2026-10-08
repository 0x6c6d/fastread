package state

import (
	"fmt"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/timing"
	"github.com/0x6c6d/fastread/internal/tokenize"
)

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }

var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func testTokens() []tokenize.Token {
	return []tokenize.Token{{Text: "a"}, {Text: "b"}, {Text: "c", ParaEnd: true}}
}

func TestPlayerAdvance(t *testing.T) {
	clk := &fakeClock{now: t0}
	toks := testTokens()
	p := NewPlayer(Config{Tokens: toks, WPM: 300, Size: 3, Clock: clk})
	p.Start()
	da := timing.DelayPara("a", 300, false)
	db := timing.DelayPara("b", 300, false)
	if got, want := p.Deadline(), t0.Add(da); !got.Equal(want) {
		t.Fatalf("deadline = %v, want %v", got, want)
	}

	if p.Tick() || p.Index() != 0 {
		t.Fatalf("tick at t0: index %d", p.Index())
	}
	clk.now = t0.Add(da - time.Nanosecond)
	if p.Tick() || p.Index() != 0 {
		t.Fatalf("tick before deadline: index %d", p.Index())
	}
	clk.now = t0.Add(da)
	if p.Tick() || p.Index() != 1 {
		t.Fatalf("tick at deadline: index %d", p.Index())
	}
	if got, want := p.Deadline(), t0.Add(da+db); !got.Equal(want) {
		t.Fatalf("deadline = %v, want %v", got, want)
	}

	clk.now = t0.Add(da + db + time.Second)
	if !p.Tick() {
		t.Fatal("late tick: finished = false")
	}
	if p.Index() != 2 || !p.Finished() {
		t.Fatalf("index %d finished %v", p.Index(), p.Finished())
	}
	if !p.Tick() {
		t.Fatal("tick after finish must keep returning true")
	}
}

func TestPlayerQuit(t *testing.T) {
	tests := []struct {
		name string
		act  Action
		quit bool
	}{
		{"none", ActNone, false},
		{"pause", ActTogglePause, false},
		{"wpm up", ActWPMUp, false},
		{"wpm down", ActWPMDown, false},
		{"size down", ActSizeDown, false},
		{"size up", ActSizeUp, false},
		{"prev", ActPrev, false},
		{"next", ActNext, false},
		{"home", ActHome, false},
		{"progress", ActToggleProgress, false},
		{"help", ActToggleHelp, false},
		{"quit", ActQuit, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPlayer(Config{Tokens: testTokens(), Start: 1, WPM: 300, Size: 3, Clock: &fakeClock{now: t0}})
			if got := p.Apply(tc.act); got != tc.quit {
				t.Fatalf("Apply(%v) = %v, want %v", tc.act, got, tc.quit)
			}
		})
	}
}

func TestPlayerStartClamp(t *testing.T) {
	tests := []struct {
		name  string
		start int
		want  int
	}{
		{"negative", -3, 0},
		{"zero", 0, 0},
		{"middle", 1, 1},
		{"too big", 99, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPlayer(Config{Tokens: testTokens(), Start: tc.start, WPM: 300, Size: 3, Clock: &fakeClock{now: t0}})
			if p.Index() != tc.want {
				t.Fatalf("Index = %d, want %d", p.Index(), tc.want)
			}
		})
	}
	p := NewPlayer(Config{Tokens: testTokens(), WPM: 300, Size: 3})
	p.Start()
	_ = p.Tick()
	if p.Len() != 3 {
		t.Fatalf("Len = %d", p.Len())
	}
}

func keyTokens() []tokenize.Token {
	toks := make([]tokenize.Token, 30)
	for i := range toks {
		toks[i] = tokenize.Token{Text: fmt.Sprintf("w%d", i)}
	}
	toks[29].ParaEnd = true
	return toks
}

func TestPlayerKeys(t *testing.T) {
	type want struct {
		index, wpm, size    int
		playing, prog, help bool
	}
	tests := []struct {
		name      string
		wpm, size int
		start     int
		prog      bool
		pause     bool
		acts      []Action
		want      want
	}{
		{"starts playing", 300, 3, 0, true, false, nil, want{0, 300, 3, true, true, false}},
		{"space twice", 300, 3, 0, true, false, []Action{ActTogglePause, ActTogglePause}, want{0, 300, 3, true, true, false}},
		{"space once", 300, 3, 0, true, false, []Action{ActTogglePause}, want{0, 300, 3, false, true, false}},
		{"up", 300, 3, 0, true, false, []Action{ActWPMUp}, want{0, 325, 3, true, true, false}},
		{"up to max", 1490, 3, 0, true, false, []Action{ActWPMUp}, want{0, 1500, 3, true, true, false}},
		{"up at max", 1500, 3, 0, true, false, []Action{ActWPMUp}, want{0, 1500, 3, true, true, false}},
		{"down", 300, 3, 0, true, false, []Action{ActWPMDown}, want{0, 275, 3, true, true, false}},
		{"down to min", 60, 3, 0, true, false, []Action{ActWPMDown}, want{0, 50, 3, true, true, false}},
		{"down at min", 50, 3, 0, true, false, []Action{ActWPMDown}, want{0, 50, 3, true, true, false}},
		{"size up", 300, 2, 0, true, false, []Action{ActSizeUp}, want{0, 300, 3, true, true, false}},
		{"size up at max", 300, 5, 0, true, false, []Action{ActSizeUp}, want{0, 300, 5, true, true, false}},
		{"size down", 300, 2, 0, true, false, []Action{ActSizeDown}, want{0, 300, 1, true, true, false}},
		{"size down at min", 300, 1, 0, true, false, []Action{ActSizeDown}, want{0, 300, 1, true, true, false}},
		{"next playing", 300, 3, 0, true, false, []Action{ActNext}, want{10, 300, 3, true, true, false}},
		{"next playing clamp", 300, 3, 25, true, false, []Action{ActNext}, want{29, 300, 3, true, true, false}},
		{"prev playing clamp", 300, 3, 5, true, false, []Action{ActPrev}, want{0, 300, 3, true, true, false}},
		{"next paused", 300, 3, 0, true, true, []Action{ActNext}, want{1, 300, 3, false, true, false}},
		{"prev paused at 0", 300, 3, 0, true, true, []Action{ActPrev}, want{0, 300, 3, false, true, false}},
		{"prev paused", 300, 3, 4, true, true, []Action{ActPrev}, want{3, 300, 3, false, true, false}},
		{"next paused at end", 300, 3, 29, true, true, []Action{ActNext}, want{29, 300, 3, false, true, false}},
		{"home playing", 300, 3, 17, true, false, []Action{ActHome}, want{0, 300, 3, true, true, false}},
		{"home paused", 300, 3, 17, true, true, []Action{ActHome}, want{0, 300, 3, false, true, false}},
		{"progress from true", 300, 3, 0, true, false, []Action{ActToggleProgress}, want{0, 300, 3, true, false, false}},
		{"progress from false", 300, 3, 0, false, false, []Action{ActToggleProgress}, want{0, 300, 3, true, true, false}},
		{"help once", 300, 3, 0, true, false, []Action{ActToggleHelp}, want{0, 300, 3, true, true, true}},
		{"help twice", 300, 3, 0, true, false, []Action{ActToggleHelp, ActToggleHelp}, want{0, 300, 3, true, true, false}},
		{"quit changes nothing", 300, 3, 4, true, false, []Action{ActQuit}, want{4, 300, 3, true, true, false}},
		{"none and unknown", 300, 3, 4, true, false, []Action{ActNone, Action(99), Action(-1)}, want{4, 300, 3, true, true, false}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clk := &fakeClock{now: t0}
			p := NewPlayer(Config{Tokens: keyTokens(), Start: tc.start, WPM: tc.wpm, Size: tc.size, ShowProgress: tc.prog, Clock: clk})
			if tc.pause {
				p.Apply(ActTogglePause)
			}
			for _, a := range tc.acts {
				if quit := p.Apply(a); quit != (a == ActQuit) {
					t.Fatalf("Apply(%v) = %v", a, quit)
				}
			}
			got := want{p.Index(), p.WPM(), p.Size(), p.Playing(), p.ShowProgress(), p.ShowHelp()}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}

	newP := func() (*Player, *fakeClock) {
		clk := &fakeClock{now: t0}
		return NewPlayer(Config{Tokens: keyTokens(), WPM: 300, Size: 3, Clock: clk}), clk
	}
	d := func(i, wpm int) time.Duration {
		tk := keyTokens()[i]
		return timing.DelayPara(tk.Text, wpm, tk.ParaEnd)
	}

	t.Run("pause and resume keeps remaining", func(t *testing.T) {
		p, clk := newP()
		if d(0, 300) != 200*time.Millisecond {
			t.Fatalf("delay = %v", d(0, 300))
		}
		clk.now = t0.Add(50 * time.Millisecond)
		p.Apply(ActTogglePause)
		clk.now = t0.Add(5 * time.Second)
		p.Apply(ActTogglePause)
		if got, want := p.Deadline(), t0.Add(5*time.Second+150*time.Millisecond); !got.Equal(want) {
			t.Fatalf("deadline = %v, want %v", got, want)
		}
	})
	t.Run("tick while paused", func(t *testing.T) {
		p, clk := newP()
		clk.now = t0.Add(50 * time.Millisecond)
		p.Apply(ActTogglePause)
		clk.now = t0.Add(10 * time.Second)
		if p.Tick() || p.Index() != 0 {
			t.Fatalf("paused tick advanced: index %d", p.Index())
		}
	})
	t.Run("wpm change keeps deadline", func(t *testing.T) {
		p, clk := newP()
		clk.now = t0.Add(50 * time.Millisecond)
		p.Apply(ActWPMUp)
		if got, want := p.Deadline(), t0.Add(200*time.Millisecond); !got.Equal(want) {
			t.Fatalf("deadline = %v, want %v", got, want)
		}
		clk.now = t0.Add(200 * time.Millisecond)
		p.Tick()
		if got, want := p.Deadline(), t0.Add(200*time.Millisecond+d(1, 325)); !got.Equal(want) {
			t.Fatalf("next deadline = %v, want %v", got, want)
		}
	})
	t.Run("next playing restarts timer", func(t *testing.T) {
		p, clk := newP()
		clk.now = t0.Add(50 * time.Millisecond)
		p.Apply(ActNext)
		if p.Index() != 10 {
			t.Fatalf("index %d", p.Index())
		}
		if got, want := p.Deadline(), t0.Add(50*time.Millisecond+d(10, 300)); !got.Equal(want) {
			t.Fatalf("deadline = %v, want %v", got, want)
		}
	})
	t.Run("next paused then resume", func(t *testing.T) {
		p, clk := newP()
		p.Apply(ActTogglePause)
		p.Apply(ActNext)
		clk.now = t0.Add(3 * time.Second)
		p.Apply(ActTogglePause)
		if got, want := p.Deadline(), clk.now.Add(d(1, 300)); !got.Equal(want) {
			t.Fatalf("deadline = %v, want %v", got, want)
		}
	})
	t.Run("finished ignores actions", func(t *testing.T) {
		p, clk := newP()
		p.Apply(ActNext)
		p.Apply(ActNext)
		p.Apply(ActNext)
		clk.now = t0.Add(time.Hour)
		if !p.Tick() || !p.Finished() {
			t.Fatal("not finished")
		}
		idx := p.Index()
		p.Apply(ActHome)
		p.Apply(ActNext)
		p.Apply(ActTogglePause)
		if p.Index() != idx || !p.Playing() {
			t.Fatalf("state changed after finish: index %d playing %v", p.Index(), p.Playing())
		}
		if !p.Apply(ActQuit) {
			t.Fatal("quit must still return true")
		}
	})
}
