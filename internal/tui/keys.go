package tui

import (
	"time"

	"github.com/0x6c6d/fastread/internal/state"
)

// EscTimeout is how long a lone ESC waits for the rest of an escape sequence; the loop calls
// Flush when no byte arrived within it.
const EscTimeout = 50 * time.Millisecond

const (
	keyEsc     = 0x1b
	keyCtrlC   = 0x03
	keyMaxBuf  = 32
	keyFinalLo = 0x40
	keyFinalHi = 0x7e
)

// KeyDecoder turns raw terminal bytes into actions. The zero value is ready to use; it is
// not safe for concurrent use.
type KeyDecoder struct {
	buf  []byte // unfinished sequence, starting with ESC
	skip bool   // discarding an overlong sequence up to its final byte
}

// Feed decodes b after any bytes kept from earlier calls and returns the complete actions in
// input order. An unfinished escape sequence at the end is kept for the next call.
func (d *KeyDecoder) Feed(b []byte) []state.Action {
	var out []state.Action
	for _, c := range b {
		out = d.step(out, c)
	}
	return out
}

// Pending reports whether an unfinished escape sequence (or a lone ESC) is buffered.
func (d *KeyDecoder) Pending() bool { return len(d.buf) > 0 }

// Flush ends a pending sequence: a lone ESC yields ActQuit (the Esc key); an unfinished
// longer sequence is dropped. Afterwards Pending is false.
func (d *KeyDecoder) Flush() []state.Action {
	var out []state.Action
	if len(d.buf) == 1 {
		out = append(out, state.ActQuit)
	}
	d.buf = d.buf[:0]
	return out
}

func isFinal(c byte) bool { return c >= keyFinalLo && c <= keyFinalHi }

func (d *KeyDecoder) step(out []state.Action, c byte) []state.Action {
	if c == keyCtrlC {
		d.buf = d.buf[:0]
		d.skip = false
		return append(out, state.ActQuit)
	}
	if d.skip {
		if isFinal(c) {
			d.skip = false
		}
		return out
	}
	switch len(d.buf) {
	case 0:
		return d.plain(out, c)
	case 1:
		switch c {
		case '[', 'O':
			d.buf = append(d.buf, c)
			return out
		case keyEsc:
			return append(out, state.ActQuit) // buf stays a lone ESC
		}
		d.buf = d.buf[:0]
		return d.plain(append(out, state.ActQuit), c)
	}
	// Inside a CSI or SS3 sequence.
	csi := d.buf[1] == '['
	ok := false
	if isFinal(c) {
		ok = true
	} else if csi {
		last := d.buf[len(d.buf)-1]
		inter := last >= 0x20 && last <= 0x2f
		ok = (c >= 0x20 && c <= 0x2f) || (c >= 0x30 && c <= 0x3f && !inter)
	}
	if !ok {
		d.buf = d.buf[:0]
		return d.plain(out, c)
	}
	if len(d.buf) >= keyMaxBuf {
		d.buf = d.buf[:0]
		d.skip = !isFinal(c)
		return out
	}
	if !isFinal(c) {
		d.buf = append(d.buf, c)
		return out
	}
	seq := string(d.buf[1:]) + string(c)
	d.buf = d.buf[:0]
	switch seq {
	case "[A", "OA":
		out = append(out, state.ActWPMUp)
	case "[B", "OB":
		out = append(out, state.ActWPMDown)
	case "[C", "OC":
		out = append(out, state.ActNext)
	case "[D", "OD":
		out = append(out, state.ActPrev)
	case "[H", "OH", "[1~", "[7~":
		out = append(out, state.ActHome)
	}
	return out
}

// plain decodes c outside any sequence.
func (d *KeyDecoder) plain(out []state.Action, c byte) []state.Action {
	switch c {
	case keyEsc:
		d.buf = append(d.buf[:0], keyEsc)
	case ' ':
		out = append(out, state.ActTogglePause)
	case 'q':
		out = append(out, state.ActQuit)
	case '[':
		out = append(out, state.ActSizeDown)
	case ']':
		out = append(out, state.ActSizeUp)
	case 'p':
		out = append(out, state.ActToggleProgress)
	case '?':
		out = append(out, state.ActToggleHelp)
	}
	return out
}
