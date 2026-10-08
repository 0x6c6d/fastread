package main

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/input"
)

const fixtureDir = "../../internal/input/testdata/"

func TestPrepareFixtures(t *testing.T) {
	rows := []struct {
		file    string
		want    string
		paraEnd map[string]bool
	}{
		{"sample.txt", "Plain text sample. Second paragraph here.", map[string]bool{"sample.": true, "text": false}},
		{"sample.md", "Title Here Some emphasis, strong and under text with snake_case kept. A link text and alt words here. Use now, see there. quoted line item one item two item three Setext Heading a b c d Escaped 5 * 3 and html plus ref link too.", map[string]bool{"Here": true}},
		{"sample.epub", "One first & café Two second Three third", nil},
		{"sample.fb2", "Body Title First body paragraph. Second body paragraph. Note text.", nil},
		{"sample.pdf", "Hello PDF world. Second page here.", map[string]bool{"world.": true}},
	}
	for _, r := range rows {
		t.Run(r.file, func(t *testing.T) {
			path := fixtureDir + r.file
			o, err := parseFlags([]string{"--no-resume", path})
			if err != nil {
				t.Fatal(err)
			}
			doc, toks, err := prepare(o, strings.NewReader(""), false)
			if err != nil {
				t.Fatal(err)
			}
			texts := make([]string, len(toks))
			for i, tk := range toks {
				texts[i] = tk.Text
			}
			if got := strings.Join(texts, " "); got != r.want {
				t.Errorf("tokens = %q, want %q", got, r.want)
			}
			if !filepath.IsAbs(doc.Path) || filepath.Base(doc.Path) != r.file {
				t.Errorf("Path = %q", doc.Path)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if doc.SHA256 != sha256.Sum256(b) {
				t.Error("SHA256 mismatch")
			}
			if len(toks) == 0 || !toks[len(toks)-1].ParaEnd {
				t.Error("last token ParaEnd not true")
			}
			for _, tk := range toks {
				if want, ok := r.paraEnd[tk.Text]; ok && tk.ParaEnd != want {
					t.Errorf("ParaEnd(%q) = %v, want %v", tk.Text, tk.ParaEnd, want)
				}
			}
		})
	}

	t.Run("notext.pdf", func(t *testing.T) {
		o, err := parseFlags([]string{"--no-resume", fixtureDir + "notext.pdf"})
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = prepare(o, strings.NewReader(""), false)
		if !errors.Is(err, input.ErrNoText) {
			t.Fatalf("err = %v, want ErrNoText", err)
		}
	})
	t.Run("epub start 9", func(t *testing.T) {
		o, err := parseFlags([]string{"--no-resume", "--start", "9", fixtureDir + "sample.epub"})
		if err == nil {
			_, _, err = prepare(o, strings.NewReader(""), false)
		}
		if err == nil || exitCode(err) != 2 {
			t.Fatalf("err = %v, exit %d, want usage error", err, exitCode(err))
		}
	})
	t.Run("txt start 5", func(t *testing.T) {
		o, err := parseFlags([]string{"--no-resume", "--start", "5", fixtureDir + "sample.txt"})
		if err != nil {
			t.Fatal(err)
		}
		_, toks, err := prepare(o, strings.NewReader(""), false)
		if err != nil {
			t.Fatal(err)
		}
		if len(toks) != 6 {
			t.Errorf("tokens = %d, want 6", len(toks))
		}
	})
}
