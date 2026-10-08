package state

import (
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
			if p.Index() != 1 || p.WPM() != 300 || p.Size() != 3 || !p.Playing() {
				t.Fatalf("state changed: index %d wpm %d size %d playing %v", p.Index(), p.WPM(), p.Size(), p.Playing())
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
