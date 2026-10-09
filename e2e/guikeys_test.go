//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2EGUIKeys(t *testing.T) {
	bin := buildBinary(t)
	display := newXvfb(t)
	stateDir := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "w%03d ", i)
	}
	sb.WriteString("\n")
	book := filepath.Join(t.TempDir(), "book.txt")
	if err := os.WriteFile(book, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// run starts one GUI process, calls body with it, then expects exit 0 within 3 s of the
	// quit key (sent by body) with empty stderr, and returns the saved index.
	run := func(name string, flags []string, body func(a *guiApp)) int {
		t.Helper()
		args := append([]string{"--ui", "gui"}, flags...)
		args = append(args, "--wpm", "50", book)
		a := startGUI(t, display, guiEnv(display, stateDir), bin, args...)
		time.Sleep(500 * time.Millisecond)
		body(a)
		code := a.wait(3 * time.Second)
		if code != 0 {
			t.Fatalf("%s: exit code %d, want 0 (stderr %q)", name, code, a.stderr())
		}
		if s := a.stderr(); s != "" {
			t.Fatalf("%s: stderr not empty: %q", name, s)
		}
		idx := readIndex(t, stateDir)
		t.Logf("%s: flags %v exit %d index %d", name, flags, code, idx)
		return idx
	}

	// 1
	idx := run("run1", []string{"--no-resume"}, func(a *guiApp) {
		a.keys("space", "Home", "Right", "Right", "Right", "Left")
		a.keys("q")
	})
	if idx != 2 {
		t.Fatalf("run1: keys space Home Right Right Right Left q: want index 2, saw %d", idx)
	}

	// 2
	r := run("run2", nil, func(a *guiApp) { a.keys("Escape") })
	if r != 2 && r != 3 {
		t.Fatalf("run2: resume then Escape: want index 2 or 3, saw %d", r)
	}

	// 3
	idx = run("run3", nil, func(a *guiApp) { a.keys("Right"); a.keys("q") })
	if idx != r+10 && idx != r+11 {
		t.Fatalf("run3: Right (playing) then q: want index %d or %d, saw %d", r+10, r+11, idx)
	}

	// 4
	idx = run("run4", []string{"--start", "5"}, func(a *guiApp) {
		a.keys("space", "Home", "Right", "Right", "Right", "Right", "Right")
		a.keys("Up", "Up", "Down", "bracketright", "bracketleft", "p", "question", "p", "question",
			"x", "Prior", "F1", "shift+q")
		if !a.alive() {
			t.Fatalf("run4: process quit on a non-quit key (stderr %q)", a.stderr())
		}
		a.keys("ctrl+c")
	})
	if idx != 5 {
		t.Fatalf("run4: --start 5, space Home Right x5, other keys, ctrl+c: want index 5, saw %d", idx)
	}

	// 5
	idx = run("run5", []string{"--start", "7"}, func(a *guiApp) {
		if !a.alive() {
			t.Fatalf("run5: not alive after 500 ms")
		}
		a.keys("space", "space", "space")
		a.keys("q")
	})
	if idx != 7 && idx != 8 {
		t.Fatalf("run5: --start 7, space x3, q: want index 7 or 8, saw %d", idx)
	}
}
