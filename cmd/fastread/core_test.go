package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/0x6c6d/fastread/internal/orp"
	"github.com/0x6c6d/fastread/internal/state"
	"github.com/0x6c6d/fastread/internal/timing"
	"github.com/0x6c6d/fastread/internal/tokenize"
)

type coreClock struct{ now time.Time }

func (c *coreClock) Now() time.Time { return c.now }

func TestPrepareSanitized(t *testing.T) {
	const B = "Hello \x1b[31mred\x1b[0m world.\n\n\x1b]0;pwned\x07title ‮evil⁦ \x9b2J end\n"
	path := filepath.Join(t.TempDir(), "esc.txt")
	if err := os.WriteFile(path, []byte(B), 0o600); err != nil {
		t.Fatal(err)
	}
	full := []string{"Hello", "[31mred[0m", "world.", "]0;pwnedtitle", "evil", "�2J", "end"}
	fullPE := []bool{false, false, true, false, false, false, true}
	tests := []struct {
		name  string
		args  []string
		stdin io.Reader
		want  []string
		pe    []bool
	}{
		{"file", []string{"--no-resume", path}, nil, full, fullPE},
		{"stdin", nil, bytes.NewReader([]byte(B)), full, fullPE},
		{"raw", []string{"a\x1b[2Jb c"}, nil, []string{"a[2Jb", "c"}, []bool{false, true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o, err := parseFlags(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			stdin := tc.stdin
			if stdin == nil {
				stdin = strings.NewReader("")
			}
			_, toks, err := prepare(o, stdin, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(toks) != len(tc.want) {
				t.Fatalf("got %v, want %q", toks, tc.want)
			}
			for i, tok := range toks {
				if tok.Text != tc.want[i] || tok.ParaEnd != tc.pe[i] {
					t.Errorf("token %d = %q/%v, want %q/%v", i, tok.Text, tok.ParaEnd, tc.want[i], tc.pe[i])
				}
				if !utf8.ValidString(tok.Text) {
					t.Errorf("token %d invalid UTF-8", i)
				}
				for _, r := range tok.Text {
					if r <= 0x1f || (r >= 0x7f && r <= 0x9f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
						t.Errorf("token %d contains forbidden rune %U", i, r)
					}
				}
				if tokenize.Sanitize(tok.Text) != tok.Text {
					t.Errorf("token %d not idempotent under Sanitize", i)
				}
			}
		})
	}
}

func TestCorePipeline(t *testing.T) {
	for _, name := range []string{"sample.txt", "sample.md", "sample.epub", "sample.fb2", "sample.pdf"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "internal", "input", "testdata", name)
			o, err := parseFlags([]string{"--no-resume", path})
			if err != nil {
				t.Fatal(err)
			}
			_, toks, err := prepare(o, strings.NewReader(""), false)
			if err != nil {
				t.Fatal(err)
			}
			if len(toks) == 0 {
				t.Fatal("no tokens")
			}
			var sum time.Duration
			for i, tok := range toks {
				if ix := orp.Index(tok.Text); ix < 0 || ix >= len(orp.Clusters(tok.Text)) {
					t.Fatalf("token %d %q: orp index %d out of range", i, tok.Text, ix)
				}
				d := timing.DelayPara(tok.Text, 600, tok.ParaEnd)
				if d < 100*time.Millisecond {
					t.Fatalf("token %d %q: delay %v < 100ms", i, tok.Text, d)
				}
				sum += d
			}
			t0 := time.Unix(1_000_000, 0)
			clk := &coreClock{now: t0}
			p := state.NewPlayer(state.Config{Tokens: toks, Start: 0, WPM: 600, Size: 2, ShowProgress: true, Clock: clk})
			p.Start()
			for i := 0; i < len(toks)+10 && !p.Finished(); i++ {
				clk.now = p.Deadline()
				p.Tick()
			}
			if !p.Finished() {
				t.Fatal("player did not finish")
			}
			if p.Index() != len(toks)-1 {
				t.Errorf("Index = %d, want %d", p.Index(), len(toks)-1)
			}
			if got, want := p.Deadline(), t0.Add(sum); !got.Equal(want) {
				t.Errorf("final Deadline = %v, want %v", got.Sub(t0), sum)
			}
			if len(toks) >= 2 {
				if _, ok := p.EffectiveWPM(); !ok {
					t.Error("EffectiveWPM not ok")
				}
			}
		})
	}
}
