//go:build e2e

package e2e

import (
	"reflect"
	"testing"
	"time"
)

func TestE2EBasic(t *testing.T) {
	bin := buildBinary(t)

	check := func(t *testing.T, s *tmuxSession, want []string) {
		t.Helper()
		seen, exit := s.watchWords(want, 20*time.Second)
		if exit != 0 || !reflect.DeepEqual(seen, want) {
			t.Errorf("seen=%v exit=%d, want %v exit 0\nlast screen:\n%s", seen, exit, want, s.capture(false))
		}
	}

	t.Run("raw", func(t *testing.T) {
		cmd := "env " + shQuote(stateEnv(t)) + " " + shQuote(bin) +
			" --ui tui --no-resume --size 1 --wpm 60 " + shQuote("Hello wonderful world") +
			`; echo "EXIT:$?"; sleep 600`
		s := newTmux(t, 80, 24, cmd)
		check(t, s, []string{"Hello", "wonderful", "world"})
	})

	t.Run("stdin", func(t *testing.T) {
		cmd := "printf 'a b c' | env " + shQuote(stateEnv(t)) + " " + shQuote(bin) +
			` --no-resume --size 1 --wpm 60; echo "EXIT:$?"; sleep 600`
		s := newTmux(t, 80, 24, cmd)
		check(t, s, []string{"a", "b", "c"})
	})
}
