//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestE2ERestore(t *testing.T) {
	bin := buildBinary(t)
	words := make([]string, 30)
	for i := range words {
		words[i] = fmt.Sprintf("r%02d", i)
	}
	text := strings.Join(words, " ")
	ways := []struct {
		name string
		do   func(s *tmuxSession)
	}{
		{"q", func(s *tmuxSession) { s.sendKeys("q") }},
		{"Escape", func(s *tmuxSession) { s.sendKeys("Escape") }},
		{"C-c", func(s *tmuxSession) { s.sendKeys("C-c") }},
		{"SIGTERM", func(s *tmuxSession) { _ = syscall.Kill(s.appPID(), syscall.SIGTERM) }},
		{"SIGINT", func(s *tmuxSession) { _ = syscall.Kill(s.appPID(), syscall.SIGINT) }},
	}
	for _, w := range ways {
		t.Run(w.name, func(t *testing.T) {
			cmd := "echo MAIN-MARK; env " + shQuote(stateEnv(t)) + " " + shQuote(bin) +
				" --no-resume --size 2 --wpm 60 " + shQuote(text) +
				`; echo "EXIT:$?"; stty -a | tr ' ' '\n' | grep -x -e icanon -e -icanon -e echo -e -echo | sed 's/^/STTY:/'; sleep 600`
			s := newTmux(t, 80, 24, cmd)
			fail := func(format string, args ...any) {
				t.Helper()
				t.Errorf(format, args...)
				t.Logf("screen:\n%s", s.capture(false))
				t.FailNow()
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				alt, vis := s.paneState()
				if alt && !vis {
					break
				}
				if time.Now().After(deadline) {
					fail("never reached alternate screen with hidden cursor (alt=%v vis=%v)", alt, vis)
				}
				time.Sleep(20 * time.Millisecond)
			}
			w.do(s)
			screen, ok := s.waitFor(func(sc string) bool {
				return strings.Contains(sc, "EXIT:0") && strings.Contains(sc, "STTY:echo")
			}, 2*time.Second)
			if !ok {
				fail("no EXIT:0 within 2s")
			}
			alt, vis := s.paneState()
			if alt || !vis {
				fail("after exit: alt=%v cursorVisible=%v, want false/true", alt, vis)
			}
			for _, want := range []string{"MAIN-MARK", "EXIT:0", "STTY:icanon", "STTY:echo"} {
				if !containsLine(screen, want) {
					fail("missing line %q", want)
				}
			}
			esc := s.capture(true)
			if strings.Contains(esc, "?1049h") {
				fail("final screen contains ?1049h")
			}
			if strings.ContainsAny(esc, "▀▄█") {
				fail("final screen contains block glyphs")
			}
		})
	}
}

func containsLine(screen, want string) bool {
	for _, l := range strings.Split(screen, "\n") {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}
