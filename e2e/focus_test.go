//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/tui/glyph"
)

const focusText = "a I ab Hello wonderful reading, \"extraordinary\" internationally (wonderful) -- " +
	"naïve ¿Qué? don't 1,000,000 2026 日本語の本 한국어 Ωmega ÆØÅ “quoted” café end."

const focusASCII = "a I ab Hello wonderful reading, \"extraordinary\" internationally (wonderful) -- " +
	"don't 1,000,000 2026 end."

// stableFrames polls capture(true) every 20 ms until EXIT:0 shows up and calls fn for each frame
// that equals the previous capture. It returns false on timeout.
func stableFrames(s *tmuxSession, timeout time.Duration, fn func(raw string)) bool {
	deadline := time.Now().Add(timeout)
	prev := ""
	for time.Now().Before(deadline) {
		cur := s.capture(true)
		if strings.Contains(cur, "EXIT:0") {
			return true
		}
		if cur != "" && cur == prev {
			fn(cur)
		}
		prev = cur
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func rowText(r []cell) string {
	var b strings.Builder
	for _, c := range r {
		b.WriteString(c.Text)
	}
	return strings.TrimSpace(b.String())
}

func TestE2EFocusColumn(t *testing.T) {
	bin := buildBinary(t)

	run := func(t *testing.T, w int, text string, noColor bool, flags string) *tmuxSession {
		env := "TERM=tmux-256color COLORTERM=truecolor"
		if noColor {
			env += " NO_COLOR=1"
		}
		cmd := "unset NO_COLOR; env " + env + " " + shQuote(stateEnv(t)) + " " + shQuote(bin) +
			" --ui tui --no-resume " + flags + " " + shQuote(text) + `; echo "EXIT:$?"; sleep 600`
		return newTmux(t, w, 24, cmd)
	}

	size1 := func(t *testing.T, w int, noColor bool) {
		s := run(t, w, focusText, noColor, "--size 1 --wpm 100")
		center := w / 2
		words := map[string]bool{}
		bad := 0
		done := stableFrames(s, 60*time.Second, func(raw string) {
			rows := parseScreen(raw)
			if noColor {
				for _, r := range rows {
					for _, c := range r {
						if c.Colored && bad < 3 {
							bad++
							t.Errorf("colored cell %+v with NO_COLOR\n%s", c, raw)
						}
					}
				}
			}
			wordRow := -1
			cnt := 0
			for ri, r := range rows {
				for _, c := range r {
					if c.Focus && strings.TrimSpace(c.Text) != "" {
						wordRow = ri
						cnt++
					}
				}
			}
			if cnt == 0 {
				return
			}
			fail := func(f string, a ...any) {
				if bad < 3 {
					bad++
					t.Errorf(f+"\nframe:\n%s", append(a, raw)...)
				}
			}
			if cnt != 1 {
				fail("%d focus clusters, want 1", cnt)
				return
			}
			if cols := focusCols(rows); len(cols) != 1 || cols[0] != center {
				fail("focus columns %v, want [%d]", cols, center)
			}
			up, down := false, false
			for _, tk := range tickCols(rows) {
				if tk[1] != center {
					fail("tick at column %d, want %d", tk[1], center)
				}
				if tk[0] == wordRow-1 {
					up = true
				}
				if tk[0] == wordRow+1 {
					down = true
				}
			}
			if !up || !down {
				fail("ticks above/below word row %d missing (up=%v down=%v)", wordRow, up, down)
			}
			words[rowText(rows[wordRow])] = true
		})
		if !done {
			t.Fatalf("no EXIT:0 in time\n%s", s.capture(false))
		}
		t.Logf("distinct words observed: %d", len(words))
		if len(words) < 20 {
			t.Errorf("only %d distinct words observed: %v", len(words), words)
		}
	}

	t.Run("size1-w80", func(t *testing.T) { size1(t, 80, false) })
	t.Run("size1-w81", func(t *testing.T) { size1(t, 81, false) })

	t.Run("size3", func(t *testing.T) {
		s := run(t, 80, focusASCII, false, "--size 3 --wpm 100")
		W := glyph.Width(3)
		lo, hi := 40-W/2, 40-W/2+W
		frames := map[string]bool{}
		bad := 0
		done := stableFrames(s, 60*time.Second, func(raw string) {
			rows := parseScreen(raw)
			fc := focusCols(rows)
			if len(fc) == 0 {
				return
			}
			frames[raw] = true
			for _, c := range fc {
				if (c < lo || c >= hi) && bad < 3 {
					bad++
					t.Errorf("focus column %d outside [%d,%d)\n%s", c, lo, hi, raw)
				}
			}
			ticks := tickCols(rows)
			if len(ticks) < 2 && bad < 3 {
				bad++
				t.Errorf("fewer than two ticks: %v\n%s", ticks, raw)
			}
			for _, tk := range ticks {
				if tk[1] != 40 && bad < 3 {
					bad++
					t.Errorf("tick at column %d, want 40\n%s", tk[1], raw)
				}
			}
		})
		if !done {
			t.Fatalf("no EXIT:0 in time\n%s", s.capture(false))
		}
		t.Log(fmt.Sprintf("distinct frames: %d, glyph width %d", len(frames), W))
		if len(frames) < 10 {
			t.Errorf("only %d distinct frames", len(frames))
		}
	})

	t.Run("nocolor", func(t *testing.T) { size1(t, 80, true) })
}
