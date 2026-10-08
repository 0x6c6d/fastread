package tui

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

const sgrReset = "\x1b[0m"

// sgr returns the colour escape for style s in mode (empty: terminal default after reset).
func sgr(mode ColorMode, s Style) string {
	switch s {
	case StyleFocus:
		switch mode {
		case ColorTrue:
			return "\x1b[38;2;255;0;0m"
		case Color256:
			return "\x1b[38;5;196m"
		case Color16:
			return "\x1b[91m"
		}
		return "\x1b[1;7m"
	case StyleTick:
		// Ticks use the terminal's default foreground in every mode.
		return ""
	default:
		switch mode {
		case ColorTrue:
			return "\x1b[38;2;255;255;255m"
		case Color256:
			return "\x1b[38;5;231m"
		case Color16:
			return "\x1b[97m"
		}
		return ""
	}
}

// stripped reports whether r must never reach the terminal: C0, DEL, C1 and bidi
// embedding/override/isolate controls.
func stripped(r rune) bool {
	return r <= 0x1F || (r >= 0x7F && r <= 0x9F) ||
		(r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069)
}

// cleanCell is the last line of defence against terminal-escape injection: every invalid
// UTF-8 byte becomes U+FFFD and control runes (see stripped) are removed. It returns s
// itself, without allocating, when nothing needs to change.
func cleanCell(s string) string {
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
		if (r == utf8.RuneError && size == 1) || stripped(r) {
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
		case stripped(r):
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// Encode turns a frame into bytes for a terminal. The output consists only of cursor
// positioning, the colour SGRs, resets and cleaned cell text; malformed frames are
// clipped to W×H and never panic.
func Encode(f Frame, mode ColorMode) []byte {
	if f.W <= 0 || f.H <= 0 {
		return nil
	}
	rows := min(f.H, len(f.Cells))
	var b []byte
	for y := 0; y < rows; y++ {
		row := f.Cells[y]
		b = append(b, "\x1b["...)
		b = strconv.AppendInt(b, int64(y+1), 10)
		b = append(b, ";1H"...)
		cur := Style(255)
		for _, c := range row[:min(f.W, len(row))] {
			if c.Cont {
				continue
			}
			if c.Style != cur {
				b = append(b, sgrReset...)
				b = append(b, sgr(mode, c.Style)...)
				cur = c.Style
			}
			if t := cleanCell(c.Text); t == "" {
				b = append(b, ' ')
			} else {
				b = append(b, t...)
			}
		}
		b = append(b, sgrReset...)
	}
	return b
}
