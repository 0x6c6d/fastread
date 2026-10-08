// Package orp computes the optimal recognition point (focus cluster) of a word. It must never import UI packages.
package orp

import (
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// Clusters returns the word's extended grapheme clusters in order (nil for "").
func Clusters(word string) []string {
	if word == "" {
		return nil
	}
	var out []string
	g := uniseg.NewGraphemes(word)
	for g.Next() {
		out = append(out, g.Str())
	}
	return out
}

// isLetterDigit reports whether a cluster's first rune is a letter or a digit.
func isLetterDigit(cluster string) bool {
	r, _ := utf8.DecodeRuneInString(cluster)
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// rank maps the number of letter/digit clusters to the R9 table value.
func rank(l int) int {
	switch {
	case l <= 1:
		return 0
	case l <= 5:
		return 1
	case l <= 9:
		return 2
	case l <= 13:
		return 3
	default:
		return 4
	}
}

// fallback is the middle cluster index used when a word has no letter/digit cluster.
func fallback(n int) int {
	if n <= 0 {
		return 0
	}
	return (n - 1) / 2
}

// Position returns the R9 table value for word: the rank of the focus cluster among the
// word's letter/digit clusters. If the word has no letter/digit cluster it returns (n-1)/2
// where n is the number of grapheme clusters (0 for the empty string).
func Position(word string) int {
	cs := Clusters(word)
	l := 0
	for _, c := range cs {
		if isLetterDigit(c) {
			l++
		}
	}
	if l == 0 {
		return fallback(len(cs))
	}
	return rank(l)
}

// Index returns the 0-based grapheme-cluster index of the focus cluster within the whole
// word (leading punctuation counted): the index of the letter/digit cluster with rank
// Position(word). With no letter/digit cluster it returns (n-1)/2 (0 for "").
func Index(word string) int {
	cs := Clusters(word)
	var letters []int
	for i, c := range cs {
		if isLetterDigit(c) {
			letters = append(letters, i)
		}
	}
	if len(letters) == 0 {
		return fallback(len(cs))
	}
	return letters[rank(len(letters))]
}
