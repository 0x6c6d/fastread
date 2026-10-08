package tui

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"
)

// Terminal is the TUI's device. Run owns it and closes it before returning.
type Terminal interface {
	Read(p []byte) (int, error) // raw key bytes; returns an error once Close was called
	Write(p []byte) (int, error)
	Size() (w, h int, err error)
	MakeRaw() error
	Restore() error // undoes MakeRaw; safe to call more than once
	Close() error
}

// Escape sequences written when entering and leaving the TUI.
const (
	EnterSeq   = "\x1b[?1049h\x1b[?25l\x1b[2J" // alt screen on, cursor hidden, clear
	RestoreSeq = "\x1b[0m\x1b[?25h\x1b[?1049l" // reset SGR, cursor visible, alt screen off
)

// ttyTerminal is the real Terminal backed by /dev/tty.
type ttyTerminal struct {
	f  *os.File
	fd int // obtained once via SyscallConn; never via (*os.File).Fd

	mu    sync.Mutex
	state *term.State // non-nil while in raw mode
}

// OpenTTY opens /dev/tty read-write (keys and output always use the controlling terminal,
// even when stdin is a pipe). It returns an error when there is no controlling terminal.
func OpenTTY() (Terminal, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("tui: open /dev/tty: %w", err)
	}
	rc, err := f.SyscallConn()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("tui: /dev/tty: %w", err)
	}
	fd := -1
	if err := rc.Control(func(u uintptr) { fd = int(u) }); err != nil {
		f.Close()
		return nil, fmt.Errorf("tui: /dev/tty: %w", err)
	}
	if !term.IsTerminal(fd) {
		f.Close()
		return nil, errors.New("tui: /dev/tty is not a terminal")
	}
	return &ttyTerminal{f: f, fd: fd}, nil
}

func (t *ttyTerminal) Read(p []byte) (int, error)  { return t.f.Read(p) }
func (t *ttyTerminal) Write(p []byte) (int, error) { return t.f.Write(p) }

func (t *ttyTerminal) Size() (w, h int, err error) {
	return term.GetSize(t.fd)
}

func (t *ttyTerminal) MakeRaw() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state != nil {
		return nil
	}
	st, err := term.MakeRaw(t.fd)
	if err != nil {
		return err
	}
	t.state = st
	return nil
}

func (t *ttyTerminal) Restore() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == nil {
		return nil
	}
	err := term.Restore(t.fd, t.state)
	t.state = nil
	return err
}

func (t *ttyTerminal) Close() error { return t.f.Close() }
