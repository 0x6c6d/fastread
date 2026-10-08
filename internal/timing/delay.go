package timing

import (
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Delay returns how long word is shown at wpm words per minute. Pure function.
func Delay(word string, wpm int) time.Duration {
	return DelayPara(word, wpm, false)
}

// DelayPara is Delay with the paragraph-end flag (paraEnd = last word of a paragraph).
func DelayPara(word string, wpm int, paraEnd bool) time.Duration {
	wpm = clampWPM(wpm)
	base := 60000 / float64(wpm) // milliseconds
	p := punctMul(word)
	if paraEnd && ParagraphMul > p {
		p = ParagraphMul
	}
	bonus := 0.0
	if l := letterCount(word); l > LongWordFrom {
		bonus = math.Min(LongWordStep*float64(l-LongWordFrom), LongWordCap)
	}
	ms := math.Round(base * p * (1 + bonus))
	return time.Duration(ms) * time.Millisecond
}

// punctMul returns the punctuation multiplier from the last non-closer rune of word.
func punctMul(word string) float64 {
	w := strings.TrimRight(word, "\"')]}»”’")
	r, _ := utf8.DecodeLastRuneInString(w)
	switch r {
	case '.', '!', '?', '…':
		return SentenceMul
	case ',', ';', ':':
		return ClauseMul
	}
	return 1
}

// letterCount counts letter and digit runes in word.
func letterCount(word string) int {
	n := 0
	for _, r := range word {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}

// clampWPM limits wpm to [MinWPM, MaxWPM].
func clampWPM(wpm int) int {
	if wpm < MinWPM {
		return MinWPM
	}
	if wpm > MaxWPM {
		return MaxWPM
	}
	return wpm
}
