//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	buildMu    sync.Mutex
	buildCache = map[string]string{}
	buildDir   string
	tmuxSeq    atomic.Int64
)

// moduleRoot returns the parent of the e2e directory (the test's working dir).
func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(wd)
}

// buildBinary runs `go build <buildArgs...> -o <dir>/fastread ./cmd/fastread` from the module
// root (once per distinct buildArgs per test binary run, cached) and returns the path.
func buildBinary(t *testing.T, buildArgs ...string) string {
	t.Helper()
	buildMu.Lock()
	defer buildMu.Unlock()
	key := strings.Join(buildArgs, "\x00")
	if p, ok := buildCache[key]; ok {
		return p
	}
	if buildDir == "" {
		d, err := os.MkdirTemp("", "fastread-e2e-bin-")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		buildDir = d
	}
	dir := filepath.Join(buildDir, strconv.Itoa(len(buildCache)))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	out := filepath.Join(dir, "fastread")
	args := append([]string{"build"}, buildArgs...)
	args = append(args, "-o", out, "./cmd/fastread")
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleRoot(t)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, b)
	}
	buildCache[key] = out
	return out
}

// stateEnv returns "XDG_STATE_HOME=<t.TempDir()>" so no test touches the real state dir.
func stateEnv(t *testing.T) string {
	t.Helper()
	return "XDG_STATE_HOME=" + t.TempDir()
}

// shQuote single-quotes s for sh.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type tmuxSession struct {
	socket  string
	session string
	t       *testing.T
}

func (s *tmuxSession) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	full := append([]string{"-L", s.socket, "-f", "/dev/null"}, args...)
	b, err := exec.CommandContext(ctx, "tmux", full...).CombinedOutput()
	return string(b), err
}

// newTmux starts an isolated tmux server with one detached session of size w x h running
// `sh -c shellCmd`; t.Cleanup kills the server.
func newTmux(t *testing.T, w, h int, shellCmd string) *tmuxSession {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatalf("tmux not found in PATH: the e2e tests require tmux")
	}
	n := tmuxSeq.Add(1)
	s := &tmuxSession{
		socket:  fmt.Sprintf("fastread-e2e-%d-%d", os.Getpid(), n),
		session: "e2e",
		t:       t,
	}
	t.Cleanup(func() { _, _ = s.run("kill-server") })
	out, err := s.run("new-session", "-d", "-s", s.session,
		"-x", strconv.Itoa(w), "-y", strconv.Itoa(h), "sh", "-c", shellCmd)
	if err != nil {
		t.Fatalf("tmux new-session: %v\n%s", err, out)
	}
	return s
}

// capture returns the visible pane (`capture-pane -p`, plus `-e` when escapes is true).
func (s *tmuxSession) capture(escapes bool) string {
	args := []string{"capture-pane", "-p", "-t", s.session}
	if escapes {
		args = append(args, "-e")
	}
	out, err := s.run(args...)
	if err != nil {
		return ""
	}
	return out
}

// sendKeys sends tmux key names (e.g. "q", "Space", "C-c", "Escape").
func (s *tmuxSession) sendKeys(keys ...string) {
	s.t.Helper()
	args := append([]string{"send-keys", "-t", s.session}, keys...)
	if out, err := s.run(args...); err != nil {
		s.t.Fatalf("tmux send-keys: %v\n%s", err, out)
	}
}

// resize resizes the window.
func (s *tmuxSession) resize(w, h int) {
	s.t.Helper()
	if out, err := s.run("resize-window", "-t", s.session,
		"-x", strconv.Itoa(w), "-y", strconv.Itoa(h)); err != nil {
		s.t.Fatalf("tmux resize-window: %v\n%s", err, out)
	}
}

// waitFor polls capture(false) every 20 ms until cond is true or timeout; returns the last
// screen and whether cond became true.
func (s *tmuxSession) waitFor(cond func(screen string) bool, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	for {
		screen := s.capture(false)
		if cond(screen) {
			return screen, true
		}
		if time.Now().After(deadline) {
			return screen, false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// watchWords polls every 20 ms until a line equal to "EXIT:<n>" appears or timeout, and
// returns, in order without consecutive duplicates, every trimmed screen line that is one of
// words, plus the exit code (-1 on timeout).
func (s *tmuxSession) watchWords(words []string, timeout time.Duration) (seen []string, exit int) {
	want := map[string]bool{}
	for _, w := range words {
		want[w] = true
	}
	deadline := time.Now().Add(timeout)
	for {
		screen := s.capture(false)
		for _, line := range strings.Split(screen, "\n") {
			line = strings.TrimSpace(line)
			if code, ok := strings.CutPrefix(line, "EXIT:"); ok {
				if n, err := strconv.Atoi(code); err == nil {
					return seen, n
				}
			}
			if want[line] && (len(seen) == 0 || seen[len(seen)-1] != line) {
				seen = append(seen, line)
			}
		}
		if time.Now().After(deadline) {
			return seen, -1
		}
		time.Sleep(20 * time.Millisecond)
	}
}
