//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/tui/glyph"
)

// settle polls until two consecutive captures are identical and ok(raw) holds for them.
func (s *tmuxSession) settle(ok func(raw string) bool, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	prev, last := "", ""
	for {
		last = s.capture(true)
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

// ticksOK reports whether the screen has exactly two ticks in column col, gap rows apart, and
// all non-space focus cells inside [col-w/2, col-w/2+w).
func ticksOK(raw string, col, gap, w int) bool {
	rows := parseScreen(raw)
	tk := tickCols(rows)
	if len(tk) != 2 || tk[0][1] != col || tk[1][1] != col || tk[1][0]-tk[0][0] != gap {
		return false
	}
	lo, hi := col-w/2, col-w/2+w
	for _, c := range focusCols(rows) {
		if c < lo || c >= hi {
			return false
		}
	}
	return true
}

func TestE2EResize(t *testing.T) {
	bin := buildBinary(t)
	text := strings.TrimSpace(strings.Repeat("wonderful ", 40))
	cmd := "env TERM=tmux-256color COLORTERM=truecolor " + shQuote(stateEnv(t)) + " " + shQuote(bin) +
		" --no-resume --size 3 --wpm 60 " + shQuote(text) + `; echo "EXIT:$?"; sleep 600`
	s := newTmux(t, 80, 24, cmd)

	if _, ok := s.settle(func(raw string) bool { return len(tickCols(parseScreen(raw))) > 0 }, 10*time.Second); !ok {
		t.Fatalf("no first frame\n%s", s.capture(false))
	}
	s.sendKeys("Space")

	step := func(name string, w, h, col, size int) {
		t.Helper()
		gap := glyph.Rows(size) + 1
		gw := glyph.Width(size)
		raw, ok := s.settle(func(raw string) bool { return ticksOK(raw, col, gap, gw) }, 2*time.Second)
		if !ok {
			t.Fatalf("%s: want two ticks at column %d, %d rows apart (size %d)\n%s", name, col, gap, size, raw)
		}
	}

	step("80x24", 80, 24, 40, 3)

	s.resize(40, 8)
	step("40x8", 40, 8, 20, 2)

	s.resize(19, 4)
	raw, ok := s.settle(func(raw string) bool {
		return strings.TrimSpace(s.capture(false)) == "terminal too small" &&
			len(tickCols(parseScreen(raw))) == 0 && len(focusCols(parseScreen(raw))) == 0
	}, 2*time.Second)
	if !ok {
		t.Fatalf("19x4: want only 'terminal too small'\n%s", s.capture(false))
	}
	_ = raw

	pid := s.appPID()
	alive := func(name string) {
		t.Helper()
		time.Sleep(1 * time.Second)
		if strings.Contains(s.capture(false), "EXIT:") {
			t.Fatalf("%s: process exited\n%s", name, s.capture(false))
		}
		if _, err := os.Stat("/proc/" + strconv.Itoa(pid)); err != nil {
			t.Fatalf("%s: pid %d gone", name, pid)
		}
	}
	s.resize(1, 1)
	alive("1x1")
	s.resize(10, 3)
	alive("10x3")

	s.resize(80, 24)
	step("80x24 again", 80, 24, 40, 3)

	s.resize(121, 30)
	step("121x30", 121, 30, 60, 3)

	s.sendKeys("q")
	if scr, ok := s.waitFor(func(sc string) bool { return strings.Contains(sc, "EXIT:0") }, 2*time.Second); !ok {
		t.Fatalf("no EXIT:0 after q\n%s", fmt.Sprint(scr))
	}
}
