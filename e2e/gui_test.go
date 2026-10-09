//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestE2EGUIXvfb(t *testing.T) {
	bin := buildBinary(t)
	display := newXvfb(t)
	raw := strings.TrimSpace(strings.Repeat("hello ", 60))

	// start runs one app with its own state dir; on failure it logs stderr and timings.
	start := func(t *testing.T, wantWindow bool, args ...string) (*guiApp, string) {
		t.Helper()
		stateDir := t.TempDir()
		t0 := time.Now()
		t.Cleanup(func() {
			if t.Failed() {
				t.Logf("elapsed %v", time.Since(t0))
			}
		})
		var a *guiApp
		if wantWindow {
			a = startGUI(t, display, guiEnv(display, stateDir), bin, args...)
		} else {
			a = launchGUI(t, display, guiEnv(display, stateDir), bin, args...)
		}
		t.Cleanup(func() {
			if t.Failed() {
				t.Logf("stderr: %q", a.stderr())
			}
		})
		return a, stateDir
	}

	for _, tc := range []struct{ name, key string }{
		{"quit-q", "q"}, {"quit-escape", "Escape"}, {"quit-ctrl-c", "ctrl+c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := start(t, true, "--ui", "gui", "--no-resume", "--wpm", "60", raw)
			if w, h := a.geometry(); w != 600 || h != 300 {
				t.Fatalf("geometry %dx%d, want 600x300", w, h)
			}
			time.Sleep(2 * time.Second)
			if !a.alive() {
				t.Fatalf("exited early")
			}
			a.keys(tc.key)
			if code := a.wait(3 * time.Second); code != 0 {
				t.Fatalf("exit code %d, want 0", code)
			}
			if s := a.stderr(); s != "" {
				t.Fatalf("stderr not empty: %q", s)
			}
		})
	}

	book := filepath.Join(t.TempDir(), "book.txt")
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "w%03d ", i)
	}
	if err := os.WriteFile(book, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		sig  syscall.Signal
	}{{"sigterm", syscall.SIGTERM}, {"sigint", syscall.SIGINT}} {
		t.Run(tc.name, func(t *testing.T) {
			a, stateDir := start(t, true, "--ui", "gui", "--no-resume", "--wpm", "300", book)
			time.Sleep(1500 * time.Millisecond)
			a.signal(tc.sig)
			if code := a.wait(2 * time.Second); code != 0 {
				t.Fatalf("exit code %d, want 0", code)
			}
			if idx := readIndex(t, stateDir); idx < 1 {
				t.Fatalf("saved index %d, want >= 1", idx)
			}
		})
	}

	t.Run("end-of-text", func(t *testing.T) {
		a, _ := start(t, false, "--ui", "gui", "--no-resume", "--wpm", "1500", "a b c")
		if code := a.wait(8 * time.Second); code != 0 {
			t.Fatalf("exit code %d, want 0", code)
		}
	})

	t.Run("resize", func(t *testing.T) {
		a, _ := start(t, true, "--ui", "gui", "--no-resume", "--wpm", "60", raw)
		a.resize(801, 401)
		time.Sleep(300 * time.Millisecond)
		if w, h := a.geometry(); w != 801 || h != 401 {
			t.Fatalf("geometry %dx%d, want 801x401", w, h)
		}
		a.resize(1, 1)
		time.Sleep(time.Second)
		if !a.alive() {
			t.Fatalf("exited after 1x1 resize")
		}
		a.resize(600, 300)
		time.Sleep(300 * time.Millisecond)
		a.keys("q")
		if code := a.wait(3 * time.Second); code != 0 {
			t.Fatalf("exit code %d, want 0", code)
		}
	})
}
