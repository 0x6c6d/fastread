//go:build e2e

package e2e

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// paneState returns tmux's #{alternate_on} and #{cursor_flag} for the session's pane.
func (s *tmuxSession) paneState() (altScreen, cursorVisible bool) {
	out, err := s.run("display-message", "-p", "-t", s.session, "#{alternate_on} #{cursor_flag}")
	if err != nil {
		return false, false
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return false, false
	}
	return f[0] == "1", f[1] == "1"
}

// appPID returns the PID of the fastread process running in the pane (a child of
// #{pane_pid} whose comm is "fastread"); it polls up to 5 s and fails the test if none.
func (s *tmuxSession) appPID() int {
	s.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, err := s.run("display-message", "-p", "-t", s.session, "#{pane_pid}")
		if pp, perr := strconv.Atoi(strings.TrimSpace(out)); err == nil && perr == nil {
			path := "/proc/" + strconv.Itoa(pp) + "/task/" + strconv.Itoa(pp) + "/children"
			if b, rerr := os.ReadFile(path); rerr == nil {
				for _, c := range strings.Fields(string(b)) {
					comm, cerr := os.ReadFile("/proc/" + c + "/comm")
					if cerr == nil && strings.TrimSpace(string(comm)) == "fastread" {
						pid, _ := strconv.Atoi(c)
						return pid
					}
				}
			}
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("fastread process not found in pane")
			return 0
		}
		time.Sleep(20 * time.Millisecond)
	}
}
