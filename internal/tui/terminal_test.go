package tui

import (
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// openPty opens a pty pair; the master is non-blocking (pollable, supports deadlines).
// It returns the master as a file, its descriptor, and the slave as a file.
func openPty(t *testing.T) (master *os.File, mfd int, slave *os.File) {
	t.Helper()
	mfd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open /dev/ptmx: %v", err)
	}
	if err := unix.IoctlSetPointerInt(mfd, unix.TIOCSPTLCK, 0); err != nil {
		unix.Close(mfd)
		t.Fatalf("unlock pty: %v", err)
	}
	n, err := unix.IoctlGetInt(mfd, unix.TIOCGPTN)
	if err != nil {
		unix.Close(mfd)
		t.Fatalf("pty number: %v", err)
	}
	if err := unix.SetNonblock(mfd, true); err != nil {
		unix.Close(mfd)
		t.Fatalf("nonblock: %v", err)
	}
	master = os.NewFile(uintptr(mfd), "ptmx")
	t.Cleanup(func() { master.Close() })
	slave, err = os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open slave: %v", err)
	}
	return master, mfd, slave
}

func termiosFlags(t *testing.T, fd int) (icanon, echo bool) {
	t.Helper()
	tio, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		t.Fatalf("TCGETS: %v", err)
	}
	return tio.Lflag&unix.ICANON != 0, tio.Lflag&unix.ECHO != 0
}

func TestTTYPty(t *testing.T) {
	master, mfd, slave := openPty(t)
	tt, err := newTTY(slave)
	if err != nil {
		t.Fatalf("newTTY: %v", err)
	}
	var _ Terminal = tt
	var _ Resizer = tt
	t.Cleanup(func() { tt.Close() })

	if err := unix.IoctlSetWinsize(mfd, unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 80}); err != nil {
		t.Fatalf("TIOCSWINSZ: %v", err)
	}
	if w, h, err := tt.Size(); err != nil || w != 80 || h != 24 {
		t.Fatalf("Size = (%d, %d, %v), want (80, 24, nil)", w, h, err)
	}

	if err := tt.MakeRaw(); err != nil {
		t.Fatalf("MakeRaw: %v", err)
	}
	if ic, ec := termiosFlags(t, tt.fd); ic || ec {
		t.Fatalf("after MakeRaw ICANON %v ECHO %v, want both off", ic, ec)
	}
	if err := tt.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if ic, ec := termiosFlags(t, tt.fd); !ic || !ec {
		t.Fatalf("after Restore ICANON %v ECHO %v, want both on", ic, ec)
	}
	if err := tt.Restore(); err != nil {
		t.Fatalf("second Restore: %v", err)
	}

	if _, err := tt.Write([]byte("hi")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	master.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 16)
	got := ""
	for len(got) < 2 {
		n, err := master.Read(buf)
		got += string(buf[:n])
		if err != nil {
			break
		}
	}
	if got != "hi" {
		t.Fatalf("master read %q, want %q", got, "hi")
	}

	if err := tt.MakeRaw(); err != nil {
		t.Fatalf("MakeRaw: %v", err)
	}
	if _, err := master.Write([]byte("q")); err != nil {
		t.Fatalf("master write: %v", err)
	}
	type rd struct {
		s   string
		err error
	}
	rch := make(chan rd, 1)
	go func() {
		b := make([]byte, 8)
		n, err := tt.Read(b)
		rch <- rd{string(b[:n]), err}
	}()
	select {
	case r := <-rch:
		if r.s != "q" || r.err != nil {
			t.Fatalf("Read = (%q, %v), want (\"q\", nil)", r.s, r.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Read did not return the byte written to the master")
	}
	if err := tt.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// SIGWINCH reaches Resized.
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatalf("kill: %v", err)
	}
	select {
	case <-tt.Resized():
	case <-time.After(time.Second):
		t.Fatal("Resized did not receive within 1s after SIGWINCH")
	}

	// A blocked Read returns an error after Close.
	go func() {
		b := make([]byte, 8)
		n, err := tt.Read(b)
		rch <- rd{string(b[:n]), err}
	}()
	time.Sleep(50 * time.Millisecond)
	if err := tt.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case r := <-rch:
		if r.err == nil {
			t.Fatalf("Read after Close = (%q, nil), want an error", r.s)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked Read did not return within 1s after Close")
	}

	// After Close a further SIGWINCH neither blocks nor panics.
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatalf("kill: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := tt.Close(); err != nil && err != os.ErrClosed {
		t.Fatalf("second Close: %v", err)
	}
}
