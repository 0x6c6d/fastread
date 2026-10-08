package tui

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

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

// Resizer is implemented by terminals that report size changes. The channel receives
// (coalesced, never blocking the sender) after each change; Run redraws on it.
type Resizer interface{ Resized() <-chan struct{} }

// ttyTerminal is the real Terminal backed by /dev/tty.
type ttyTerminal struct {
	f  *os.File
	fd int // obtained once via SyscallConn; never via (*os.File).Fd

	sig     chan os.Signal // SIGWINCH, buffered (cap 1)
	resized chan struct{}  // coalesced resize notifications, buffered (cap 1)
	stop    chan struct{}  // closed by Close; ends the forwarding goroutine

	mu     sync.Mutex
	state  *term.State // non-nil while in raw mode
	closed bool
}

// OpenTTY opens /dev/tty read-write (keys and output always use the controlling terminal,
// even when stdin is a pipe). It returns an error when there is no controlling terminal.
func OpenTTY() (Terminal, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("tui: open /dev/tty: %w", err)
	}
	t, err := newTTY(f)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// newTTY wraps the terminal device f (closed on error) and registers for SIGWINCH.
func newTTY(f *os.File) (*ttyTerminal, error) {
	rc, err := f.SyscallConn()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("tui: %s: %w", f.Name(), err)
	}
	fd := -1
	if err := rc.Control(func(u uintptr) { fd = int(u) }); err != nil {
		f.Close()
		return nil, fmt.Errorf("tui: %s: %w", f.Name(), err)
	}
	if !term.IsTerminal(fd) {
		f.Close()
		return nil, errors.New("tui: " + f.Name() + " is not a terminal")
	}
	t := &ttyTerminal{
		f:       f,
		fd:      fd,
		sig:     make(chan os.Signal, 1),
		resized: make(chan struct{}, 1),
		stop:    make(chan struct{}),
	}
	signal.Notify(t.sig, syscall.SIGWINCH)
	go t.forward()
	return t, nil
}

// forward turns SIGWINCH deliveries into coalesced Resized notifications until Close.
func (t *ttyTerminal) forward() {
	for {
		select {
		case <-t.stop:
			return
		case <-t.sig:
			select {
			case t.resized <- struct{}{}:
			default:
			}
		}
	}
}

// Resized implements Resizer.
func (t *ttyTerminal) Resized() <-chan struct{} { return t.resized }

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

// Close releases the SIGWINCH registration and closes the device; it is idempotent.
func (t *ttyTerminal) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()
	signal.Stop(t.sig)
	close(t.stop)
	return t.f.Close()
}
