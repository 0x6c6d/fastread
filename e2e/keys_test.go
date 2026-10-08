//go:build e2e

package e2e

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/tui/glyph"
)

var keyWordRe = regexp.MustCompile(`k[0-9]{3}`)

// keyWord returns the first kNNN word on the screen as a number, or -1.
func keyWord(screen string) int {
	m := keyWordRe.FindString(screen)
	if m == "" {
		return -1
	}
	n, _ := strconv.Atoi(m[1:])
	return n
}

// stable polls up to timeout for two consecutive identical captures satisfying ok.
func (s *tmuxSession) stable(ok func(screen string) bool, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	prev, last := "", ""
	for {
		last = s.capture(false)
		if last != "" && last == prev && ok(last) {
			return last, true
		}
		prev = last
		if time.Now().After(deadline) {
			return last, false
		}
		time.Sleep(30 * time.Millisecond)
	}
}

func TestE2EKeys(t *testing.T) {
	bin := buildBinary(t)
	words := make([]string, 200)
	for i := range words {
		words[i] = fmt.Sprintf("k%03d", i)
	}
	text := strings.Join(words, " ")
	cmd := "env TERM=tmux-256color COLORTERM=truecolor " + shQuote(stateEnv(t)) + " " + shQuote(bin) +
		" --no-resume --size 1 --wpm 50 " + shQuote(text) + `; echo "EXIT:$?"; sleep 600`
	s := newTmux(t, 80, 24, cmd)

	expect := func(step string, ok func(screen string) bool) string {
		t.Helper()
		scr, good := s.stable(ok, 2*time.Second)
		if !good {
			t.Fatalf("%s: expectation not met\nscreen:\n%s", step, scr)
		}
		return scr
	}
	wordIs := func(n int) func(string) bool { return func(sc string) bool { return keyWord(sc) == n } }

	expect("first word k000", wordIs(0))
	s.sendKeys("Space")
	time.Sleep(100 * time.Millisecond)
	scr := expect("paused", func(sc string) bool { return keyWord(sc) >= 0 })
	n := keyWord(scr)

	for _, k := range []string{"PPage", "F1", "x", "Q"} {
		s.sendKeys(k)
	}
	time.Sleep(500 * time.Millisecond)
	expect("other keys ignored", func(sc string) bool { return keyWord(sc) == n && !strings.Contains(sc, "EXIT:") })

	s.sendKeys("Right")
	expect("Right -> n+1", wordIs(n+1))
	s.sendKeys("Right")
	expect("Right -> n+2", wordIs(n+2))
	s.sendKeys("Left")
	expect("Left -> n+1", wordIs(n+1))
	s.sendKeys("Home")
	expect("Home -> k000", wordIs(0))

	ticks := func(gap int) func(string) bool {
		return func(sc string) bool {
			raw := s.capture(true)
			tk := tickCols(parseScreen(raw))
			return len(tk) == 2 && tk[0][1] == 40 && tk[1][1] == 40 && tk[1][0]-tk[0][0] == gap
		}
	}
	s.sendKeys("]")
	expect("size 2", ticks(glyph.Rows(2)+1))
	s.sendKeys("[")
	expect("size 1", ticks(glyph.Rows(1)+1))
	expect("progress shown", func(sc string) bool { return strings.Contains(sc, "word 1/200") })

	s.sendKeys("p")
	expect("progress hidden", func(sc string) bool { return !strings.Contains(sc, "word 1/") })
	s.sendKeys("p")
	expect("progress shown again", func(sc string) bool { return strings.Contains(sc, "word 1/200") })

	s.sendKeys("?")
	expect("help on", func(sc string) bool { return strings.HasPrefix(strings.Split(sc, "\n")[0], "space pause") })
	s.sendKeys("?")
	expect("help off", func(sc string) bool { return strings.TrimSpace(strings.Split(sc, "\n")[0]) == "" })

	s.sendKeys("Up")
	s.sendKeys("Down")
	time.Sleep(300 * time.Millisecond)
	expect("Up/Down keep running", func(sc string) bool { return keyWord(sc) == 0 && !strings.Contains(sc, "EXIT:") })

	s.sendKeys("Space")
	s.sendKeys("Right")
	expect("Right while playing jumps 10", func(sc string) bool { return keyWord(sc) >= 10 })

	s.sendKeys("Escape")
	expect("Escape exits 0", func(sc string) bool { return strings.Contains(sc, "EXIT:0") })
	if alt, cur := s.paneState(); alt || !cur {
		t.Fatalf("pane alt=%v cursor=%v, want main screen and visible cursor\n%s", alt, cur, s.capture(false))
	}
}
