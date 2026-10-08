package state

import (
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/timing"
	"github.com/0x6c6d/fastread/internal/tokenize"
)

const ms = time.Millisecond

func stepTokens() []tokenize.Token {
	w := strings.Repeat("abcdefghij", 6)
	return []tokenize.Token{{Text: "aa"}, {Text: w + "."}, {Text: "bb", ParaEnd: true}}
}

func stepP3() []string {
	w := strings.Repeat("abcdefghij", 6)
	return []string{w[:20] + "-", w[20:40] + "-", w[40:] + "."}
}

func sixParts() []string {
	w := strings.Repeat("abcdefghij", 6)
	var ps []string
	for i := 0; i < 6; i++ {
		s := w[i*10 : i*10+10]
		if i < 5 {
			s += "-"
		} else {
			s += "."
		}
		ps = append(ps, s)
	}
	return ps
}

// newStepPlayer returns a player on token 1 (the long word) at t0+200ms.
func newStepPlayer(t *testing.T) (*Player, *fakeClock) {
	t.Helper()
	clk := &fakeClock{now: t0}
	p := NewPlayer(Config{Tokens: stepTokens(), WPM: 300, Size: 3, Clock: clk})
	clk.now = t0.Add(200 * ms)
	if p.Tick() || p.Index() != 1 || p.Parts() != 1 {
		t.Fatalf("setup: index %d parts %d", p.Index(), p.Parts())
	}
	return p, clk
}

func TestPlayerSteps(t *testing.T) {
	t.Run("walk through three steps", func(t *testing.T) {
		p, clk := newStepPlayer(t)
		p.SetParts(stepP3())
		if p.Parts() != 3 || p.Part() != 0 || !p.Deadline().Equal(t0.Add(500*ms)) {
			t.Fatalf("after SetParts: parts %d part %d deadline %v", p.Parts(), p.Part(), p.Deadline().Sub(t0))
		}
		p.SetParts(stepP3())
		if !p.Deadline().Equal(t0.Add(500 * ms)) {
			t.Errorf("equal parts changed deadline: %v", p.Deadline().Sub(t0))
		}
		steps := []struct {
			at         time.Duration
			part, idx  int
			parts      int
			wantDeadln time.Duration
		}{
			{500, 1, 1, 3, 800},
			{800, 2, 1, 3, 1400},
			{1400, 0, 2, 1, 1900},
		}
		for _, s := range steps {
			clk.now = t0.Add(s.at * ms)
			if p.Tick() {
				t.Fatalf("finished early at %v", s.at)
			}
			if p.Index() != s.idx || p.Part() != s.part || p.Parts() != s.parts || !p.Deadline().Equal(t0.Add(s.wantDeadln*ms)) {
				t.Fatalf("at %v: idx %d part %d parts %d deadline %v", s.at, p.Index(), p.Part(), p.Parts(), p.Deadline().Sub(t0))
			}
		}
		clk.now = t0.Add(1900 * ms)
		if !p.Tick() || !p.Finished() {
			t.Fatal("not finished at t0+1900ms")
		}
		p.SetParts(stepP3())
		if p.Parts() != 1 {
			t.Error("SetParts after finish changed parts")
		}
	})

	t.Run("equal parts after Up key", func(t *testing.T) {
		p, _ := newStepPlayer(t)
		p.SetParts(stepP3())
		p.Apply(ActWPMUp)
		d := p.Deadline()
		p.SetParts(stepP3())
		if !p.Deadline().Equal(d) {
			t.Errorf("deadline changed: %v -> %v", d, p.Deadline())
		}
	})

	t.Run("nil equals whole word", func(t *testing.T) {
		p, _ := newStepPlayer(t)
		d := p.Deadline()
		p.SetParts(nil)
		p.SetParts([]string{p.Token().Text})
		if !p.Deadline().Equal(d) || p.Parts() != 1 {
			t.Errorf("deadline %v parts %d", p.Deadline(), p.Parts())
		}
	})

	t.Run("replace keeps time spent playing", func(t *testing.T) {
		p, clk := newStepPlayer(t)
		p.SetParts(stepP3())
		clk.now = t0.Add(300 * ms)
		p.SetParts(sixParts())
		if p.Parts() != 6 || !p.Deadline().Equal(t0.Add(420*ms)) {
			t.Errorf("parts %d deadline %v", p.Parts(), p.Deadline().Sub(t0))
		}
	})

	t.Run("replace keeps time spent paused", func(t *testing.T) {
		p, clk := newStepPlayer(t)
		p.SetParts(stepP3())
		clk.now = t0.Add(300 * ms)
		p.Apply(ActTogglePause)
		p.SetParts(sixParts())
		clk.now = t0.Add(5 * time.Second)
		p.Apply(ActTogglePause)
		if want := t0.Add(5*time.Second + 120*ms); !p.Deadline().Equal(want) {
			t.Errorf("deadline %v, want %v", p.Deadline(), want)
		}
	})

	t.Run("SetParts nil at last step", func(t *testing.T) {
		p, clk := newStepPlayer(t)
		p.SetParts(stepP3())
		clk.now = t0.Add(800 * ms)
		p.Tick()
		if p.Part() != 2 {
			t.Fatalf("part %d", p.Part())
		}
		p.SetParts(nil)
		if p.Part() != 0 || p.Parts() != 1 {
			t.Errorf("part %d parts %d", p.Part(), p.Parts())
		}
	})

	t.Run("jump resets steps", func(t *testing.T) {
		p, clk := newStepPlayer(t)
		p.SetParts(stepP3())
		clk.now = t0.Add(800 * ms)
		p.Tick()
		p.Apply(ActNext)
		if p.Index() != 2 || p.Part() != 0 || p.Parts() != 1 {
			t.Errorf("index %d part %d parts %d", p.Index(), p.Part(), p.Parts())
		}
	})

	t.Run("late tick crosses steps", func(t *testing.T) {
		p, clk := newStepPlayer(t)
		p.SetParts(stepP3())
		clk.now = t0.Add(1450 * ms)
		if p.Tick() || p.Index() != 2 || p.Part() != 0 {
			t.Errorf("index %d part %d", p.Index(), p.Part())
		}
	})

	t.Run("effective wpm counts split word once", func(t *testing.T) {
		p, clk := newStepPlayer(t)
		p.SetParts(stepP3())
		clk.now = t0.Add(1400 * ms)
		p.Tick()
		// words completed: "aa" and the split word = 2 in 1.4 s.
		got, ok := p.EffectiveWPM()
		if !ok || got != 86 {
			t.Errorf("EffectiveWPM = %d, %v; want 86, true", got, ok)
		}
	})
}

func chunkToken(text string) []string {
	r := []rune(text)
	if len(r) <= 20 {
		return []string{text}
	}
	var out []string
	for len(r) > 20 {
		out = append(out, string(r[:20])+"-")
		r = r[20:]
	}
	return append(out, string(r))
}

func TestPlayerStepsNoDrift(t *testing.T) {
	w := strings.Repeat("abcdefghij", 6)
	plain := []string{"word", "word,", "word."}
	toks := make([]tokenize.Token, 50)
	for i := range toks {
		if i%5 == 4 {
			toks[i].Text = w
		} else {
			toks[i].Text = plain[i%3]
		}
	}
	toks[49].ParaEnd = true
	clk := &fakeClock{now: t0}
	const wpm = 600
	p := NewPlayer(Config{Tokens: toks, WPM: wpm, Size: 3, Clock: clk})

	stepDelay := func() time.Duration {
		parts := chunkToken(p.Token().Text)
		i := p.Part()
		if i < len(parts)-1 {
			return timing.Delay(strings.TrimSuffix(parts[i], "-"), wpm)
		}
		return timing.DelayPara(parts[i], wpm, p.Token().ParaEnd)
	}
	var done time.Duration // delays of completed steps
	jitter := []time.Duration{0, 3, 7, 9}
	p.SetParts(chunkToken(p.Token().Text))
	for n := 0; ; n++ {
		if n > 1000 {
			t.Fatal("did not finish")
		}
		if want := t0.Add(done + stepDelay()); !p.Deadline().Equal(want) {
			t.Fatalf("step %d: deadline %v, want %v", n, p.Deadline().Sub(t0), want.Sub(t0))
		}
		done += stepDelay()
		clk.now = p.Deadline().Add(jitter[n%4] * ms)
		if p.Tick() {
			break
		}
		p.SetParts(chunkToken(p.Token().Text))
	}
	if !p.Finished() || p.Index() != 49 {
		t.Errorf("finished %v index %d", p.Finished(), p.Index())
	}
}
