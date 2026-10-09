//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// freeDisplay returns the first ":N" (N in 900-999) with no X socket or lock file.
func freeDisplay(t *testing.T) string {
	t.Helper()
	for n := 900; n <= 999; n++ {
		_, e1 := os.Stat(fmt.Sprintf("/tmp/.X11-unix/X%d", n))
		_, e2 := os.Stat(fmt.Sprintf("/tmp/.X%d-lock", n))
		if os.IsNotExist(e1) && os.IsNotExist(e2) {
			return fmt.Sprintf(":%d", n)
		}
	}
	t.Fatalf("no free display number in 900-999")
	return ""
}

// cleanEnv is os.Environ() without display, state and runtime-dir variables, plus extra.
func cleanEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "DISPLAY=") || strings.HasPrefix(kv, "WAYLAND_DISPLAY=") ||
			strings.HasPrefix(kv, "XDG_STATE_HOME=") || strings.HasPrefix(kv, "XDG_RUNTIME_DIR=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

// runCmd runs bin with args under a timeout; returns stdout, stderr, exit code and duration.
func runCmd(t *testing.T, timeout time.Duration, env []string, bin string, args ...string) (string, string, int, time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	start := time.Now()
	err := cmd.Run()
	d := time.Since(start)
	if ctx.Err() != nil {
		t.Fatalf("%s timed out after %v\nstderr: %s", bin, timeout, se.String())
	}
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run %s: %v", bin, err)
		}
		code = ee.ExitCode()
	}
	return so.String(), se.String(), code, d
}

// oneLine checks stderr is exactly one line starting with "fastread: ".
func oneLine(t *testing.T, stderr string) {
	t.Helper()
	if !strings.HasSuffix(stderr, "\n") || strings.Count(stderr, "\n") != 1 || !strings.HasPrefix(stderr, "fastread: ") {
		t.Fatalf("stderr must be exactly one line starting with \"fastread: \", got %q", stderr)
	}
}

func TestE2EGUINoDisplay(t *testing.T) {
	bin := buildBinary(t)
	for _, name := range []string{"raw", "file"} {
		t.Run(name, func(t *testing.T) {
			state := filepath.Join(t.TempDir(), "state")
			arg := "x"
			if name == "file" {
				arg = filepath.Join(t.TempDir(), "b.txt")
				if err := os.WriteFile(arg, []byte("hello world\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			stdout, stderr, code, d := runCmd(t, 5*time.Second, cleanEnv("XDG_STATE_HOME="+state), bin, "--ui", "gui", arg)
			if code != 1 {
				t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
			}
			if d > 5*time.Second {
				t.Fatalf("took %v", d)
			}
			oneLine(t, stderr)
			if stdout != "" {
				t.Fatalf("stdout not empty: %q", stdout)
			}
			if _, err := os.Stat(state); !os.IsNotExist(err) {
				t.Fatalf("state dir created (stat err %v)", err)
			}
		})
	}
}

func TestE2EGUIBadDisplay(t *testing.T) {
	bin := buildBinary(t)
	cases := map[string]func() []string{
		"x11": func() []string { return []string{"DISPLAY=" + freeDisplay(t)} },
		"wayland": func() []string {
			return []string{fmt.Sprintf("WAYLAND_DISPLAY=fastread-e2e-none-%d", os.Getpid())}
		},
	}
	for _, name := range []string{"x11", "wayland"} {
		t.Run(name, func(t *testing.T) {
			rt := t.TempDir()
			if err := os.Chmod(rt, 0o700); err != nil {
				t.Fatal(err)
			}
			extra := append(cases[name](), "XDG_RUNTIME_DIR="+rt, "XDG_STATE_HOME="+t.TempDir())
			_, stderr, code, _ := runCmd(t, 10*time.Second, cleanEnv(extra...), bin, "--ui", "gui", "hello world")
			if code != 1 {
				t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
			}
			oneLine(t, stderr)
			if strings.Contains(stderr, "panic") || strings.Contains(stderr, "goroutine ") {
				t.Fatalf("panic or stack trace: %q", stderr)
			}
		})
	}
}

func TestE2ENoguiBinary(t *testing.T) {
	root := moduleRoot(t)
	bin := filepath.Join(t.TempDir(), "fastread")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-tags", "nogui", "-o", bin, "./cmd/fastread")
	build.Dir = root
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CGO_ENABLED=0")
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("nogui build: %v\n%s", err, b)
	}
	for _, args := range [][]string{{"version", "-m", bin}, {"tool", "nm", bin}} {
		c, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
		cmd := exec.CommandContext(c, "go", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
		out, err := cmd.CombinedOutput()
		cancel2()
		if err != nil {
			t.Fatalf("go %v: %v\n%s", args, err, out)
		}
		if bytes.Contains(out, []byte("gioui.org")) {
			t.Fatalf("go %v output mentions gioui.org", args)
		}
	}
	for name, extra := range map[string][]string{
		"nodisplay": nil,
		"display":   {"DISPLAY=" + freeDisplay(t)},
	} {
		t.Run(name, func(t *testing.T) {
			env := cleanEnv(append(extra, "XDG_STATE_HOME="+t.TempDir())...)
			_, stderr, code, _ := runCmd(t, 10*time.Second, env, bin, "--ui", "gui", "x")
			if code != 1 {
				t.Fatalf("exit %d, want 1", code)
			}
			if strings.Count(stderr, "\n") != 1 || !strings.Contains(stderr, "GUI not available") {
				t.Fatalf("stderr %q: want one line containing \"GUI not available\"", stderr)
			}
		})
	}
	cmd := fmt.Sprintf("%s %s --no-resume --size 1 --wpm 120 'a b c'; echo EXIT:$?; sleep 30",
		stateEnv(t), shQuote(bin))
	s := newTmux(t, 80, 24, cmd)
	seen, exit := s.watchWords([]string{"a", "b", "c"}, 20*time.Second)
	if exit != 0 || strings.Join(seen, ",") != "a,b,c" {
		t.Fatalf("seen %v exit %d, want a,b,c exit 0\n%s", seen, exit, s.capture(false))
	}
}
