package tui

import "strings"

// ColorMode selects how colours are encoded.
type ColorMode int

const (
	ColorNone ColorMode = iota // NO_COLOR set: focus is bold+reverse, no colour escapes
	Color16
	Color256
	ColorTrue
)

// DetectColorMode: NO_COLOR non-empty -> ColorNone; else COLORTERM "truecolor" or "24bit"
// -> ColorTrue; else TERM containing "256color" -> Color256; else Color16.
func DetectColorMode(getenv func(string) string) ColorMode {
	if getenv("NO_COLOR") != "" {
		return ColorNone
	}
	if ct := getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		return ColorTrue
	}
	if strings.Contains(getenv("TERM"), "256color") {
		return Color256
	}
	return Color16
}
