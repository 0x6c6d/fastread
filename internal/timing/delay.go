package timing

import "time"

// Delay returns how long word is shown at wpm words per minute. Pure function.
func Delay(word string, wpm int) time.Duration {
	return DelayPara(word, wpm, false)
}

// DelayPara is Delay with the paragraph-end flag (paraEnd = last word of a paragraph).
func DelayPara(word string, wpm int, paraEnd bool) time.Duration {
	wpm = clampWPM(wpm)
	base := time.Duration(float64(time.Minute) / float64(wpm))
	// TODO(phase3): multipliers (punctuation, long word, paragraph end) are not applied yet;
	// word and paraEnd are accepted and ignored for now.
	_, _ = word, paraEnd
	return base.Round(time.Millisecond)
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
