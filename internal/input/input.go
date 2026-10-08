// Package input selects and loads the text source. It must never import UI packages.
package input

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Document is loaded text split into paragraphs.
type Document struct {
	Paragraphs []string
	Path       string   // absolute path of a file source; "" for raw text and stdin
	SHA256     [32]byte // SHA-256 of the file bytes; zero for raw text and stdin
}

// Kind identifies where the text comes from.
type Kind int

const (
	KindRaw   Kind = iota // the positional argument is the text
	KindStdin             // read stdin
	KindFile              // the positional argument is an existing file
)

// Source is the selected text source.
type Source struct {
	Kind Kind
	Text string // KindRaw: the argument
	Path string // KindFile: the argument as given
}

// Load returns the document for src. KindRaw: ParseText of the text. KindStdin: reads all of
// stdin, then ParseText. KindFile: LoadFile of the path. Read errors are wrapped with %w.
// Stdin and files are capped at MaxInputBytes (ErrTooLarge).
func Load(src Source, stdin io.Reader) (Document, error) {
	return load(src, stdin, DefaultLimits())
}

// load is Load with explicit limits (tests pass small values).
func load(src Source, stdin io.Reader, lim Limits) (Document, error) {
	switch src.Kind {
	case KindRaw:
		return Document{Paragraphs: ParseText([]byte(src.Text))}, nil
	case KindStdin:
		if stdin == nil {
			return Document{}, fmt.Errorf("read stdin: no reader: %w", ErrNoInput)
		}
		b, err := readCapped(stdin, lim.MaxBytes)
		if err != nil {
			return Document{}, fmt.Errorf("read stdin: %w", err)
		}
		return Document{Paragraphs: ParseText(b)}, nil
	case KindFile:
		return loadFile(src.Path, lim)
	default:
		return Document{}, fmt.Errorf("unknown source kind %d: %w", src.Kind, ErrUnsupported)
	}
}

var bom = []byte{0xEF, 0xBB, 0xBF}

// ParseText decodes b as UTF-8 and splits it into paragraphs:
// invalid UTF-8 bytes -> U+FFFD; a leading BOM (EF BB BF) is dropped; CRLF and lone CR -> LF;
// a paragraph is a run of lines separated by one or more blank (whitespace-only) lines;
// lines of a paragraph are joined with "\n"; no empty or whitespace-only paragraphs.
func ParseText(b []byte) []string {
	b = bytes.TrimPrefix(b, bom)
	text := decodeUTF8(b)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var paras []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			paras = append(paras, strings.Join(cur, "\n"))
			cur = cur[:0]
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return paras
}

// decodeUTF8 returns b as a string with every invalid byte replaced by U+FFFD.
func decodeUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b) + 8)
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		sb.WriteRune(r) // RuneError (U+FFFD) for an invalid byte, size 1
		b = b[size:]
	}
	return sb.String()
}
