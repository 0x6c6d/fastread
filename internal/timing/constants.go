// Package timing computes per-word display durations. It imports only the standard
// library and must never import UI packages.
package timing

// SentenceMul multiplies the delay of a word that ends a sentence (. ! ?).
const SentenceMul = 2.0

// ClauseMul multiplies the delay of a word that ends a clause (, ; :).
const ClauseMul = 1.5

// LongWordStep is the extra delay fraction added per letter beyond LongWordFrom.
const LongWordStep = 0.05

// LongWordCap caps the total extra delay fraction for long words.
const LongWordCap = 0.5

// LongWordFrom is the word length (in letters) above which the long-word bonus applies.
const LongWordFrom = 8

// ParagraphMul multiplies the delay of the last word of a paragraph.
const ParagraphMul = 2.5

// MinWPM is the lowest supported reading speed in words per minute.
const MinWPM = 50

// MaxWPM is the highest supported reading speed in words per minute.
const MaxWPM = 1500
