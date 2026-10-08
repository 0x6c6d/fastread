// Package tokenize splits text into words with paragraph-end flags. It must never import UI packages.
package tokenize

import (
	"strings"
	"unicode"
)

// Token is one displayed word. ParaEnd is true for the last word of a paragraph
// (and therefore for the last word of the whole text).
type Token struct {
	Text    string
	ParaEnd bool
}

// Tokenize splits every paragraph on Unicode whitespace (unicode.IsSpace) into tokens.
// Punctuation stays attached to its word. Paragraphs that contain no word add nothing.
// The result is nil or empty when there is no word at all.
func Tokenize(paragraphs []string) []Token {
	var tokens []Token
	for _, p := range paragraphs {
		words := strings.FieldsFunc(p, unicode.IsSpace)
		if len(words) == 0 {
			continue
		}
		for i, w := range words {
			tokens = append(tokens, Token{Text: w, ParaEnd: i == len(words)-1})
		}
	}
	return tokens
}
