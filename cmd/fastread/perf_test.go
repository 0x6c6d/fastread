package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/fastread/internal/input"
	"github.com/0x6c6d/fastread/internal/tokenize"
)

const perfBound = 2 * time.Second

// perfText builds deterministic UTF-8 prose of at least size bytes and returns
// it with the exact number of whitespace-separated words.
func perfText(size int) (data []byte, words int) {
	special := []string{"naïve", "café", "über", "日本"}
	puncts := []string{"", "", ",", ".", ";", "!"}
	var b strings.Builder
	b.Grow(size + 64)
	for b.Len() < size {
		var w string
		if words%7 == 3 {
			w = special[(words/7)%len(special)]
		} else {
			n := words%15 + 1
			w = strings.Repeat(string(rune('a'+words%26)), n)
		}
		b.WriteString(w)
		b.WriteString(puncts[words%len(puncts)])
		words++
		switch {
		case words%120 == 0:
			b.WriteString("\n\n")
		default:
			b.WriteByte(' ')
		}
	}
	return []byte(b.String()), words
}

func TestPerfLoadTokenize5MB(t *testing.T) {
	data, words := perfText(5 << 20)
	path := filepath.Join(t.TempDir(), "big.txt")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	bound := perfBound
	if raceEnabled {
		bound *= 10
	}
	rows := []struct {
		name string
		load func() (input.Document, error)
	}{
		{"file", func() (input.Document, error) { return input.LoadFile(path) }},
		{"stdin", func() (input.Document, error) {
			return input.Load(input.Source{Kind: input.KindStdin}, bytes.NewReader(data))
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			start := time.Now()
			doc, err := r.load()
			if err != nil {
				t.Fatal(err)
			}
			tokens := tokenize.Tokenize(doc.Paragraphs)
			d := time.Since(start)
			t.Logf("%s: load+tokenize %d words in %v", r.name, len(tokens), d)
			if d > bound {
				t.Fatalf("took %v, bound %v", d, bound)
			}
			if len(tokens) != words {
				t.Fatalf("got %d tokens, want %d", len(tokens), words)
			}
			if len(tokens) == 0 || !tokens[len(tokens)-1].ParaEnd {
				t.Fatal("last token must have ParaEnd")
			}
		})
	}
}

func BenchmarkLoadTokenize5MB(b *testing.B) {
	data, _ := perfText(5 << 20)
	path := filepath.Join(b.TempDir(), "big.txt")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc, err := input.LoadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		tokenize.Tokenize(doc.Paragraphs)
	}
}
