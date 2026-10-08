//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

var wordRe = regexp.MustCompile(`w[0-9]{4}`)

// shownWord returns the first wNNNN word on the screen as a number, or -1.
func shownWord(screen string) int {
	m := wordRe.FindString(screen)
	if m == "" {
		return -1
	}
	n, _ := strconv.Atoi(m[1:])
	return n
}

// writeBook writes the 200-word fixture and returns its symlink-resolved absolute path.
func writeBook(t *testing.T) string {
	t.Helper()
	words := make([]string, 200)
	for i := range words {
		words[i] = fmt.Sprintf("w%04d", i)
	}
	p := filepath.Join(t.TempDir(), "book.txt")
	if err := os.WriteFile(p, []byte(strings.Join(words, " ")+"\n"), 0o644); err != nil {
		t.Fatalf("write book: %v", err)
	}
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	return r
}

// entry requires exactly one *.json in <stateDir>/fastread and decodes it.
func entry(t *testing.T, stateDir string) (path string, index int) {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(stateDir, "fastread", "*.json"))
	if len(files) != 1 {
		t.Fatalf("want exactly one entry in %s/fastread, got %v", stateDir, files)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	var e struct {
		Path  string `json:"path"`
		Index int    `json:"index"`
	}
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("decode entry %q: %v", b, err)
	}
	return e.Path, e.Index
}

// startRun launches the binary in a fresh tmux session; extra is the shell-quoted argument
// string, stdinPrefix an optional "printf ... | " prefix, errFile the stderr redirect target.
func startRun(t *testing.T, bin, env, stdinPrefix, extra, errFile string) *tmuxSession {
	t.Helper()
	cmd := stdinPrefix + "env " + shQuote(env) + " " + shQuote(bin) + " " + extra +
		" 2>" + shQuote(errFile) + `; echo "EXIT:$?"; sleep 600`
	return newTmux(t, 80, 24, cmd)
}

func waitWord(t *testing.T, s *tmuxSession, pred func(n int) bool, timeout time.Duration) int {
	t.Helper()
	screen, ok := s.waitFor(func(sc string) bool { return pred(shownWord(sc)) }, timeout)
	if !ok {
		t.Fatalf("expected word never shown\nlast screen:\n%s", screen)
	}
	return shownWord(screen)
}

func waitExit(t *testing.T, s *tmuxSession, timeout time.Duration) {
	t.Helper()
	_, exit := s.watchWords(nil, timeout)
	if exit != 0 {
		t.Fatalf("exit=%d, want 0\nlast screen:\n%s", exit, s.capture(false))
	}
}

func TestE2ESIGTERMSavesState(t *testing.T) {
	bin := buildBinary(t)
	for _, tc := range []struct {
		name string
		sig  syscall.Signal
	}{{"SIGTERM", syscall.SIGTERM}, {"SIGINT", syscall.SIGINT}} {
		t.Run(tc.name, func(t *testing.T) {
			env := stateEnv(t)
			stateDir := strings.TrimPrefix(env, "XDG_STATE_HOME=")
			book := writeBook(t)
			s := startRun(t, bin, env, "", "--size 1 --wpm 50 "+shQuote(book), filepath.Join(t.TempDir(), "err"))
			screen, ok := s.waitFor(func(sc string) bool { return strings.Contains(sc, "w0002") }, 10*time.Second)
			if !ok {
				t.Fatalf("w0002 never shown\nlast screen:\n%s", screen)
			}
			k := shownWord(s.capture(false))
			pid := s.appPID()
			start := time.Now()
			if err := syscall.Kill(pid, tc.sig); err != nil {
				t.Fatalf("kill: %v", err)
			}
			_, exit := s.watchWords(nil, time.Second)
			ms := time.Since(start).Milliseconds()
			t.Logf("EXIT seen %d ms after %s", ms, tc.name)
			if exit != 0 {
				t.Fatalf("exit=%d within 1s, want 0\nlast screen:\n%s", exit, s.capture(false))
			}
			dir := filepath.Join(stateDir, "fastread")
			di, err := os.Stat(dir)
			if err != nil || di.Mode().Perm() != 0o700 {
				t.Errorf("state dir stat %v err %v, want 0700", di, err)
			}
			p, idx := entry(t, stateDir)
			files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
			raw, _ := os.ReadFile(files[0])
			if fi, err := os.Stat(files[0]); err != nil || fi.Mode().Perm() != 0o600 {
				t.Errorf("entry mode %v err %v, want 0600", fi, err)
			}
			if p != book {
				t.Errorf("entry path %q, want %q", p, book)
			}
			if idx != k && idx != k+1 {
				t.Errorf("entry index %d, want %d or %d\nentry: %s", idx, k, k+1, raw)
			}
			if strings.Contains(string(raw), "w0") {
				t.Errorf("entry contains word text: %s", raw)
			}
			if alt, cur := s.paneState(); alt || !cur {
				t.Errorf("pane alt=%v cursor=%v, want main screen and visible cursor", alt, cur)
			}
		})
	}
}

func TestE2EResume(t *testing.T) {
	bin := buildBinary(t)
	env := stateEnv(t)
	stateDir := strings.TrimPrefix(env, "XDG_STATE_HOME=")
	book := writeBook(t)
	errFile := filepath.Join(t.TempDir(), "stderr")
	base := "--size 1 --wpm 100 "

	// (1) quit at w0003.
	s := startRun(t, bin, env, "", base+shQuote(book), errFile)
	waitWord(t, s, func(n int) bool { return n >= 3 }, 15*time.Second)
	s.sendKeys("q")
	waitExit(t, s, 3*time.Second)
	_, i := entry(t, stateDir)
	if i < 3 {
		t.Fatalf("step 1: saved index %d, want >= 3", i)
	}

	// (2) resume shows word i first.
	s = startRun(t, bin, env, "", base+shQuote(book), errFile)
	if n := waitWord(t, s, func(n int) bool { return n >= 0 }, 10*time.Second); n != i {
		t.Fatalf("step 2: first word %d, want %d\nscreen:\n%s", n, i, s.capture(false))
	}
	s.sendKeys("Escape")
	waitExit(t, s, 3*time.Second)

	// (3) --start 5 overrides.
	s = startRun(t, bin, env, "", base+"--start 5 "+shQuote(book), errFile)
	if n := waitWord(t, s, func(n int) bool { return n >= 0 }, 10*time.Second); n != 5 {
		t.Fatalf("step 3: first word %d, want 5\nscreen:\n%s", n, s.capture(false))
	}
	s.sendKeys("q")
	waitExit(t, s, 3*time.Second)

	// (4) --no-resume starts at 0.
	s = startRun(t, bin, env, "", base+"--no-resume "+shQuote(book), errFile)
	if n := waitWord(t, s, func(n int) bool { return n >= 0 }, 10*time.Second); n != 0 {
		t.Fatalf("step 4: first word %d, want 0\nscreen:\n%s", n, s.capture(false))
	}
	s.sendKeys("q")
	waitExit(t, s, 3*time.Second)
	if _, i := entry(t, stateDir); i > 2 {
		t.Fatalf("step 4: saved index %d, want <= 2", i)
	}

	// (5) changed content: start at 0, no warning.
	f, err := os.OpenFile(book, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open book: %v", err)
	}
	if _, err := f.WriteString(" w0200"); err != nil {
		t.Fatalf("append: %v", err)
	}
	f.Close()
	_ = os.Remove(errFile)
	s = startRun(t, bin, env, "", base+shQuote(book), errFile)
	if n := waitWord(t, s, func(n int) bool { return n >= 0 }, 10*time.Second); n != 0 {
		t.Fatalf("step 5: first word %d, want 0\nscreen:\n%s", n, s.capture(false))
	}
	s.sendKeys("q")
	waitExit(t, s, 3*time.Second)
	if b, _ := os.ReadFile(errFile); strings.Contains(strings.ToLower(string(b)), "warning") {
		t.Errorf("step 5: stderr has warning: %s", b)
	}

	// (6) play to the end: entry deleted.
	s = startRun(t, bin, env, "", "--size 1 --wpm 1500 --start 195 "+shQuote(book), errFile)
	waitExit(t, s, 15*time.Second)
	if files, _ := filepath.Glob(filepath.Join(stateDir, "fastread", "*.json")); len(files) != 0 {
		b, _ := os.ReadFile(files[0])
		t.Fatalf("step 6: entry left: %v %s", files, b)
	}

	// (7) raw text and stdin never create state.
	env2 := stateEnv(t)
	s = startRun(t, bin, env2, "", "--size 1 --wpm 100 'xq1 yq2 zq3'", errFile)
	if screen, ok := s.waitFor(func(sc string) bool { return strings.Contains(sc, "xq1") }, 10*time.Second); !ok {
		t.Fatalf("step 7: xq1 never shown\nscreen:\n%s", screen)
	}
	s.sendKeys("q")
	waitExit(t, s, 3*time.Second)
	emptyDir(t, env2)

	s = startRun(t, bin, env2, "printf 'a b c' | ", "--size 1 --wpm 1500", errFile)
	waitExit(t, s, 10*time.Second)
	emptyDir(t, env2)
}
