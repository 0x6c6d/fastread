package tui

import "strconv"

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

// Encode turns a frame into bytes for a terminal.
func Encode(f Frame, mode ColorMode) []byte {
	var b []byte
	for y, row := range f.Cells {
		b = append(b, "\x1b["+strconv.Itoa(y+1)+";1H"...)
		cur := Style(255)
		for _, c := range row {
			if c.Cont {
				continue
			}
			if c.Style != cur {
				b = append(b, "\x1b[0m"...)
				b = append(b, sgr(mode, c.Style)...)
				cur = c.Style
			}
			if c.Text == "" {
				b = append(b, ' ')
			} else {
				b = append(b, c.Text...)
			}
		}
		b = append(b, "\x1b[0m"...)
	}
	return b
}
