//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2EInputTypes(t *testing.T) {
	fixture := func(name string) string {
		p, err := filepath.Abs(filepath.Join("..", "internal", "input", "testdata", name))
		if err != nil {
			t.Fatalf("abs: %v", err)
		}
		return p
	}

	type tcase struct {
		name  string
		file  string // fixture name, or ""
		raw   string
		stdin string
		words string
	}
	tcases := []tcase{
		{name: "raw", raw: "Hello wonderful world", words: "Hello wonderful world"},
		{name: "stdin", stdin: "one two three", words: "one two three"},
		{name: "txt", file: "sample.txt", words: "Plain text sample. Second paragraph here."},
		{name: "md", file: "sample.md", words: "Title Here Some emphasis, strong and under text with snake_case kept. A link text and alt words here. Use now, see there. quoted line item one item two item three Setext Heading a b c d Escaped 5 * 3 and html plus ref link too."},
		{name: "epub", file: "sample.epub", words: "One first & café Two second Three third"},
		{name: "fb2", file: "sample.fb2", words: "Body Title First body paragraph. Second body paragraph. Note text."},
		{name: "pdf", file: "sample.pdf", words: "Hello PDF world. Second page here."},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			bin := buildBinary(t)
			env := stateEnv(t)
			stateDir := strings.TrimPrefix(env, "XDG_STATE_HOME=")
			errFile := filepath.Join(t.TempDir(), "err.txt")
			base := shQuote(bin) + " --size 1 --wpm 300"
			var cmd string
			switch {
			case tc.file != "":
				cmd = "env " + shQuote(env) + " " + base + " " + shQuote(fixture(tc.file))
			case tc.stdin != "":
				cmd = "printf '" + tc.stdin + "' | env " + shQuote(env) + " " + base
			default:
				cmd = "env " + shQuote(env) + " " + base + " " + shQuote(tc.raw)
			}
			shell := cmd + " 2>" + shQuote(errFile) + `; echo "EXIT:$?"; sleep 600`
			s := newTmux(t, 80, 24, shell)
			words := strings.Fields(tc.words)
			// words are matched per screen line; each token is one word.
			seen, code := s.watchWords(words, 40*time.Second)
			output := strings.Join(seen, " ")
			if b, _ := os.ReadFile(errFile); len(b) > 0 {
				output += " stderr=" + string(b)
			}
			t.Logf("CASE name=%s exit=%d output=%q", tc.name, code, output)
			fail := func(format string, a ...any) {
				t.Helper()
				t.Logf("last screen:\n%s", s.capture(false))
				t.Errorf(format, a...)
			}
			if code != 0 {
				fail("exit = %d, want 0", code)
			}
			if got := strings.Join(seen, " "); got != tc.words {
				fail("seen = %q, want %q", got, tc.words)
			}
			var jsons []string
			filepath.Walk(filepath.Join(stateDir, "fastread"), func(p string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(p, ".json") {
					jsons = append(jsons, p)
				}
				return nil
			})
			if len(jsons) > 0 {
				fail("resume entries left: %v", jsons)
			}
		})
	}

	type ecase struct {
		name   string
		args   func(tmp string) []string
		code   int
		substr string
		oneLn  bool
	}
	ecases := []ecase{
		{"missing-file", func(string) []string { return []string{"./nope.txt"} }, 1, "not found", true},
		{"empty-text", func(tmp string) []string { return []string{filepath.Join(tmp, "empty.txt")} }, 1, "empty text", true},
		{"bad-flag", func(string) []string { return []string{"--wpm", "49", "hello"} }, 2, "Usage:", false},
		{"scanned-pdf", func(string) []string { return []string{fixture("notext.pdf")} }, 1, "no extractable text (scanned PDF?)", true},
		{"wrong-extension", func(tmp string) []string { return []string{filepath.Join(tmp, "book.pdf")} }, 1, "unsupported", true},
	}
	for _, ec := range ecases {
		t.Run(ec.name, func(t *testing.T) {
			bin := buildBinary(t)
			env := stateEnv(t)
			stateDir := strings.TrimPrefix(env, "XDG_STATE_HOME=")
			tmp := t.TempDir()
			if err := os.WriteFile(filepath.Join(tmp, "empty.txt"), []byte(" \n\n\t\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			epub, err := os.ReadFile(fixture("sample.epub"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(tmp, "book.pdf"), epub, 0o644); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, ec.args(tmp)...)
			cmd.Dir = tmp
			cmd.Stdin = nil // /dev/null
			cmd.Env = append(os.Environ(), env)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			runErr := cmd.Run()
			code := 0
			if runErr != nil {
				code = -1
				if ee, ok := runErr.(*exec.ExitError); ok && ctx.Err() == nil {
					code = ee.ExitCode()
				}
			}
			t.Logf("CASE name=%s exit=%d output=%q", ec.name, code, stderr.String())
			if code != ec.code {
				t.Errorf("exit = %d, want %d", code, ec.code)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout not empty: %q", stdout.String())
			}
			if !strings.HasPrefix(stderr.String(), "fastread: ") {
				t.Errorf("stderr does not start with %q: %q", "fastread: ", stderr.String())
			}
			if !strings.Contains(stderr.String(), ec.substr) {
				t.Errorf("stderr lacks %q: %q", ec.substr, stderr.String())
			}
			if ec.oneLn && strings.Count(strings.TrimRight(stderr.String(), "\n"), "\n") != 0 {
				t.Errorf("stderr is not one line: %q", stderr.String())
			}
			if _, err := os.Stat(filepath.Join(stateDir, "fastread")); err == nil {
				t.Errorf("state directory was created")
			}
		})
	}
}
