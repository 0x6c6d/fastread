package input

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// pathSuffixes are the extensions (lower case) that make a whitespace-free argument path-like.
var pathSuffixes = []string{".txt", ".md", ".epub", ".fb2", ".pdf"}

// LooksLikePath reports whether arg, which is not an existing file, should be treated as a
// mistyped path rather than as raw text.
//
// Rule: arg contains no Unicode whitespace and at least one of: contains '/', starts with '.',
// starts with '~', or ends (case-insensitively) in .txt, .md, .epub, .fb2 or .pdf.
// The empty string is not path-like.
func LooksLikePath(arg string) bool {
	if arg == "" || strings.IndexFunc(arg, unicode.IsSpace) >= 0 {
		return false
	}
	if strings.Contains(arg, "/") || strings.HasPrefix(arg, ".") || strings.HasPrefix(arg, "~") {
		return true
	}
	lower := strings.ToLower(arg)
	for _, suf := range pathSuffixes {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	return false
}

// Select applies R1/R7 source selection. hasArg reports whether a positional argument was given.
// First match wins:
//   - !hasArg and stdinIsTTY: error wrapping ErrNoInput (caller exits 2 with usage)
//   - !hasArg and !stdinIsTTY: KindStdin
//   - os.Stat(arg) succeeds: regular file gives KindFile; a directory or any other file type
//     (FIFO, socket, device) gives an error wrapping ErrUnsupported; nothing is opened
//   - os.Stat(arg) fails: LooksLikePath(arg) gives an error wrapping ErrNotFound, otherwise
//     KindRaw with the argument as text
//
// No tilde expansion, no "-" alias for stdin, no filesystem writes.
func Select(arg string, hasArg, stdinIsTTY bool) (Source, error) {
	if !hasArg {
		if stdinIsTTY {
			return Source{}, ErrNoInput
		}
		return Source{Kind: KindStdin}, nil
	}
	fi, err := os.Stat(arg)
	if err != nil {
		if LooksLikePath(arg) {
			return Source{}, fmt.Errorf("%s: %w", arg, ErrNotFound)
		}
		return Source{Kind: KindRaw, Text: arg}, nil
	}
	switch {
	case fi.Mode().IsRegular():
		return Source{Kind: KindFile, Path: arg}, nil
	case fi.IsDir():
		return Source{}, fmt.Errorf("%s: is a directory: %w", oneLine(arg), ErrUnsupported)
	default:
		return Source{}, fmt.Errorf("%s: not a regular file: %w", oneLine(arg), ErrUnsupported)
	}
}

// oneLine returns s unchanged unless it contains a control character (such as a newline),
// in which case it is quoted so an error message stays on one line.
func oneLine(s string) string {
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return strconv.Quote(s)
	}
	return s
}
