//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// newXvfb starts a private Xvfb on a free display (-displayfd) and returns ":N".
func newXvfb(t *testing.T) string {
	t.Helper()
	for _, tool := range []string{"Xvfb", "xdotool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("%s not found in PATH: the GUI e2e tests require it", tool)
		}
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	cmd := exec.Command("Xvfb", "-displayfd", "3", "-screen", "0", "1280x1024x24", "-nolisten", "tcp")
	cmd.ExtraFiles = []*os.File{w}
	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		t.Fatalf("start Xvfb: %v", err)
	}
	w.Close()
	waited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(waited)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-waited
		r.Close()
	})
	line := make(chan string, 1)
	go func() {
		s, _ := bufio.NewReader(r).ReadString('\n')
		line <- strings.TrimSpace(s)
	}()
	select {
	case s := <-line:
		if _, err := strconv.Atoi(s); err != nil {
			t.Fatalf("Xvfb did not report a display number (got %q)", s)
		}
		return ":" + s
	case <-time.After(10 * time.Second):
		t.Fatalf("Xvfb did not report a display within 10 s")
	}
	return ""
}

// guiEnv returns the environment for a GUI process on display with its own state dir.
func guiEnv(display, stateDir string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "DISPLAY=") || strings.HasPrefix(kv, "WAYLAND_DISPLAY=") ||
			strings.HasPrefix(kv, "XDG_STATE_HOME=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "DISPLAY="+display, "XDG_STATE_HOME="+stateDir)
}

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

type guiApp struct {
	t       *testing.T
	cmd     *exec.Cmd
	pid     int
	display string
	wid     string
	errBuf  *lockedBuf
	done    chan struct{}
	code    int
}

// startGUI starts bin args and waits for the window named "fastread".
func startGUI(t *testing.T, display string, env []string, bin string, args ...string) *guiApp {
	t.Helper()
	a := launchGUI(t, display, env, bin, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "xdotool", "search", "--sync", "--name", "^fastread$")
	cmd.Env = xEnv(display)
	out, err := cmd.Output()
	fields := strings.Fields(string(out))
	if err != nil || len(fields) == 0 {
		t.Fatalf("no window named fastread within 10 s: %v\nstderr: %s", err, a.stderr())
	}
	a.wid = fields[0]
	return a
}

// launchGUI starts the process without waiting for a window.
func launchGUI(t *testing.T, display string, env []string, bin string, args ...string) *guiApp {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	a := &guiApp{t: t, cmd: cmd, display: display, errBuf: &lockedBuf{}, done: make(chan struct{})}
	cmd.Stdout = &lockedBuf{}
	cmd.Stderr = a.errBuf
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start %s: %v", bin, err)
	}
	a.pid = cmd.Process.Pid
	go func() {
		err := cmd.Wait()
		a.code = cmd.ProcessState.ExitCode()
		if err != nil && a.code == -1 {
			a.code = -2
		}
		close(a.done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-a.done
		cancel()
	})
	return a
}

// xEnv is a minimal environment with only the Xvfb display.
func xEnv(display string) []string {
	env := []string{"DISPLAY=" + display}
	for _, k := range []string{"PATH", "HOME"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func (a *guiApp) xdotool(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "xdotool", args...)
	cmd.Env = xEnv(a.display)
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func (a *guiApp) keys(names ...string) {
	a.t.Helper()
	args := append([]string{"windowfocus", "--sync", a.wid, "key", "--delay", "60"}, names...)
	if out, err := a.xdotool(args...); err != nil {
		a.t.Fatalf("xdotool key %v: %v\n%s", names, err, out)
	}
}

// wait returns the exit code, or -1 on timeout.
func (a *guiApp) wait(d time.Duration) int {
	select {
	case <-a.done:
		return a.code
	case <-time.After(d):
		return -1
	}
}

func (a *guiApp) alive() bool {
	select {
	case <-a.done:
		return false
	default:
		return true
	}
}

// signal sends sig to the process.
func (a *guiApp) signal(sig syscall.Signal) {
	_ = syscall.Kill(a.pid, sig)
}

func (a *guiApp) geometry() (w, h int) {
	a.t.Helper()
	out, err := a.xdotool("getwindowgeometry", a.wid)
	if err != nil {
		a.t.Fatalf("getwindowgeometry: %v\n%s", err, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Geometry:"); ok {
			ws, hs, _ := strings.Cut(strings.TrimSpace(rest), "x")
			w, _ = strconv.Atoi(ws)
			h, _ = strconv.Atoi(hs)
			return w, h
		}
	}
	a.t.Fatalf("no Geometry line in: %s", out)
	return 0, 0
}

func (a *guiApp) resize(w, h int) {
	a.t.Helper()
	if out, err := a.xdotool("windowsize", a.wid, strconv.Itoa(w), strconv.Itoa(h)); err != nil {
		a.t.Fatalf("windowsize: %v\n%s", err, out)
	}
}

func (a *guiApp) stderr() string { return a.errBuf.String() }

// readIndex reads "index" from the single state file and checks its mode is 0600.
func readIndex(t *testing.T, stateDir string) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(stateDir, "fastread", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("want exactly one state file in %s/fastread, got %v (err %v)", stateDir, files, err)
	}
	fi, err := os.Stat(files[0])
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode %v, want 0600", fi.Mode().Perm())
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var st struct {
		Index int `json:"index"`
	}
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("state json: %v\n%s", err, b)
	}
	return st.Index
}
