// Package tokenize splits text into words with paragraph-end flags. It must never import UI packages.
package tokenize

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token is one displayed word. ParaEnd is true for the last word of a paragraph
// (and therefore for the last word of the whole text).
type Token struct {
	Text    string
	ParaEnd bool
}

// Tokenize turns paragraphs of text into display tokens:
//
//  1. Each input paragraph is split further into paragraphs at blank lines (a line,
//     delimited by "\n", "\r\n" or "\r", that is empty or contains only unicode.IsSpace
//     runes) and at U+2029 PARAGRAPH SEPARATOR. U+2028 LINE SEPARATOR is plain whitespace.
//  2. Each paragraph is split on unicode.IsSpace into fields; punctuation stays attached.
//  3. Each field is passed through Sanitize; fields that become empty are dropped.
//  4. ParaEnd is set on the last surviving token of each paragraph; a paragraph with no
//     surviving token adds nothing. The last token of the result therefore has ParaEnd.
//
// The input is scanned in a single pass without regular expressions. The result is nil
// or empty when there is no word at all.
func Tokenize(paragraphs []string) []Token {
	var tokens []Token
	for _, p := range paragraphs {
		tokens = tokenizeOne(tokens, p)
	}
	return tokens
}

// tokenizeOne appends the tokens of one input paragraph to tokens.
func tokenizeOne(tokens []Token, p string) []Token {
	paraStart := len(tokens) // index of the first token of the current paragraph
	endPara := func() {
		if len(tokens) > paraStart {
			tokens[len(tokens)-1].ParaEnd = true
		}
		paraStart = len(tokens)
	}
	fieldStart := -1  // byte offset of the current field, -1 when outside a field
	lineBlank := true // the current line has only whitespace so far
	emit := func(end int) {
		if fieldStart >= 0 {
			if w := Sanitize(p[fieldStart:end]); w != "" {
				tokens = append(tokens, Token{Text: w})
			}
			fieldStart = -1
		}
	}
	for i := 0; i < len(p); {
		r, size := utf8.DecodeRuneInString(p[i:])
		switch {
		case r == '\n' || r == '\r':
			emit(i)
			if r == '\r' && i+1 < len(p) && p[i+1] == '\n' {
				size = 2
			}
			if lineBlank {
				endPara()
			}
			lineBlank = true
		case r == ' ':
			emit(i)
			endPara()
			lineBlank = true
		case unicode.IsSpace(r):
			emit(i)
		default:
			lineBlank = false
			if fieldStart < 0 {
				fieldStart = i
			}
		}
		i += size
	}
	emit(len(p))
	endPara()
	return tokens
}

// removed reports whether Sanitize drops rune r.
func removed(r rune) bool {
	switch {
	case r <= 0x1F, r >= 0x7F && r <= 0x9F:
		return true
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}

// Sanitize returns s with every invalid UTF-8 byte sequence replaced by U+FFFD and these
// runes removed: C0 controls U+0000–U+001F, DEL U+007F, C1 controls U+0080–U+009F, bidi
// embedding/override/isolate controls U+202A–U+202E and U+2066–U+2069. Every other rune is
// kept unchanged (combining marks, ZWJ U+200D, LRM U+200E, emoji, U+FFFD, NBSP). Pure; when
// nothing changes it returns s without allocating.
func Sanitize(s string) string {
	// Fast path: find the first byte offset that needs a change.
	first := -1
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c <= 0x1F || c == 0x7F {
				first = i
				break
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || removed(r) {
			first = i
			break
		}
		i += size
	}
	if first < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteString(s[:first])
	for i := first; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteRune(utf8.RuneError)
		case removed(r):
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}
