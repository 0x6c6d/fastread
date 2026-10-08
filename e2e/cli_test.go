//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func emptyDir(t *testing.T, env string) {
	t.Helper()
	dir := strings.TrimPrefix(env, "XDG_STATE_HOME=")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Errorf("read state dir: %v", err)
		return
	}
	if len(ents) != 0 {
		t.Errorf("state dir %s not empty: %v", dir, ents)
	}
}

func TestE2EStdin(t *testing.T) {
	bin := buildBinary(t)

	t.Run("utf8", func(t *testing.T) {
		env := stateEnv(t)
		cmd := `printf 'alpha \377beta\n\ngamma' | env ` + shQuote(env) + " " + shQuote(bin) +
			` --no-resume --size 1 --wpm 120; echo "EXIT:$?"; sleep 600`
		s := newTmux(t, 80, 24, cmd)
		want := []string{"alpha", "�beta", "gamma"}
		seen, exit := s.watchWords(want, 20*time.Second)
		if exit != 0 || !reflect.DeepEqual(seen, want) {
			t.Errorf("seen=%q exit=%d, want %q exit 0\nlast screen:\n%s", seen, exit, want, s.capture(false))
		}
		emptyDir(t, env)
	})

	t.Run("keys", func(t *testing.T) {
		env := stateEnv(t)
		var words []string
		for i := 0; i < 200; i++ {
			words = append(words, "s"+string([]byte{byte('0' + i/100), byte('0' + i/10%10), byte('0' + i%10)}))
		}
		cmd := "printf %s " + shQuote(strings.Join(words, " ")) + " | env " + shQuote(env) + " " +
			shQuote(bin) + ` --no-resume --size 1 --wpm 60; echo "EXIT:$?"; sleep 600`
		s := newTmux(t, 80, 24, cmd)
		screen, ok := s.waitFor(func(sc string) bool {
			for _, l := range strings.Split(sc, "\n") {
				if strings.TrimSpace(l) == "s001" {
					return true
				}
			}
			return false
		}, 10*time.Second)
		if !ok {
			t.Fatalf("s001 never shown\nlast screen:\n%s", screen)
		}
		s.sendKeys("q")
		_, exit := s.watchWords(nil, 2*time.Second)
		if exit != 0 {
			t.Errorf("exit=%d, want 0 within 2s\nlast screen:\n%s", exit, s.capture(false))
		}
		emptyDir(t, env)
	})
}

func TestE2ENotFound(t *testing.T) {
	bin := buildBinary(t)
	for _, arg := range []string{"./nope.txt", "nope.epub", "sub/missing.md", "/nonexistent-fastread-dir/x.pdf"} {
		t.Run(arg, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, arg)
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), stateEnv(t))
			devnull, err := os.Open(os.DevNull)
			if err != nil {
				t.Fatal(err)
			}
			defer devnull.Close()
			cmd.Stdin = devnull
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err = cmd.Run()
			var ee *exec.ExitError
			if !errors.As(err, &ee) || ee.ExitCode() != 1 {
				t.Errorf("err=%v, want exit 1\nstderr: %q", err, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			e := stderr.String()
			if strings.Count(e, "\n") != 1 || !strings.HasSuffix(e, "\n") ||
				!strings.HasPrefix(e, "fastread: ") || !strings.Contains(e, "not found") {
				t.Errorf("stderr = %q, want one line 'fastread: ... not found ...'", e)
			}
		})
	}

	t.Run("raw word", func(t *testing.T) {
		cmd := "env " + shQuote(stateEnv(t)) + " " + shQuote(bin) +
			` --no-resume --size 1 --wpm 1500 nope; echo "EXIT:$?"; sleep 600`
		s := newTmux(t, 80, 24, cmd)
		seen, exit := s.watchWords([]string{"nope"}, 10*time.Second)
		if exit != 0 || !reflect.DeepEqual(seen, []string{"nope"}) {
			t.Errorf("seen=%q exit=%d\nlast screen:\n%s", seen, exit, s.capture(false))
		}
	})
}

func TestE2ENoArgTTY(t *testing.T) {
	bin := buildBinary(t)
	errFile := filepath.Join(t.TempDir(), "err")
	cmd := "env " + shQuote(stateEnv(t)) + " " + shQuote(bin) + " 2>" + shQuote(errFile) +
		`; echo "EXIT:$?"; sleep 600`
	s := newTmux(t, 80, 24, cmd)
	screen, ok := s.waitFor(func(sc string) bool {
		for _, l := range strings.Split(sc, "\n") {
			if strings.TrimSpace(l) == "EXIT:2" {
				return true
			}
		}
		return false
	}, 5*time.Second)
	b, _ := os.ReadFile(errFile)
	if !ok {
		t.Fatalf("EXIT:2 not shown\nlast screen:\n%s\nstderr:\n%s", screen, b)
	}
	first, _, _ := strings.Cut(string(b), "\n")
	if !strings.HasPrefix(first, "fastread: ") || !strings.Contains(string(b), "Usage: fastread") {
		t.Errorf("unexpected stderr:\n%s", b)
	}
	if strings.Contains(screen, "word 1/") {
		t.Errorf("something was played:\n%s", screen)
	}
}
