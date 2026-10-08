#!/usr/bin/env bash
# Verification for T063 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "e2e files carry the e2e build tag, are test files, vet clean; helpers declared" <<'CMD'
test -f e2e/xvfb_test.go && test -f e2e/gui_test.go || exit 1
for f in e2e/*.go; do
  case "$f" in *_test.go) ;; *) echo "$f is not a _test.go file"; exit 1;; esac
  head -1 "$f" | grep -qx '//go:build e2e' || { echo "$f lacks //go:build e2e"; exit 1; }
done
go vet -tags e2e ./e2e/ || exit 1
for s in 'func newXvfb(t *testing.T) string' 'func guiEnv(display, stateDir string) []string' \
  'func startGUI(t *testing.T, display string, env []string, bin string, args ...string) *guiApp' \
  'func (a *guiApp) keys(names ...string)' 'func (a *guiApp) wait(d time.Duration) int' \
  'func (a *guiApp) geometry() (w, h int)' 'func readIndex(t *testing.T, stateDir string) int'; do
  grep -qF "$s" e2e/xvfb_test.go || { echo "missing: $s"; exit 1; }
done
CMD
expect_ok "TestE2EGUIXvfb (7 subtests) and TestE2EBasic pass; nothing left running; real state dir untouched" 400 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
xv0="$(pgrep -u "$(id -u)" -x Xvfb | sort)"
out="$(go test -tags e2e -count=1 -timeout 300s -v -run '^(TestE2EGUIXvfb|TestE2EBasic)$' ./e2e/ 2>&1)"; rc=$?
printf '%s\n' "$out" | tail -n 60
test "$rc" -eq 0 || exit 1
for n in TestE2EGUIXvfb TestE2EGUIXvfb/quit-q TestE2EGUIXvfb/quit-escape TestE2EGUIXvfb/quit-ctrl-c TestE2EGUIXvfb/sigterm TestE2EGUIXvfb/sigint TestE2EGUIXvfb/end-of-text TestE2EGUIXvfb/resize TestE2EBasic; do
  printf '%s\n' "$out" | grep -qE -- "^ *--- PASS: $n \(" || { echo "no PASS line for $n"; exit 1; }
done
sleep 0.5
test "$(pgrep -u "$(id -u)" -x Xvfb | sort)" = "$xv0" || { echo "an Xvfb started by the test is still running"; exit 1; }
! pgrep -u "$(id -u)" -x fastread >/dev/null || { echo "fastread process left running"; exit 1; }
if ls "/tmp/tmux-$(id -u)" 2>/dev/null | grep -q '^fastread-e2e-'; then
  for s in $(ls "/tmp/tmux-$(id -u)" | grep '^fastread-e2e-'); do
    tmux -L "$s" ls >/dev/null 2>&1 && { echo "tmux server $s still running"; exit 1; }
  done
fi
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || { echo "real state dir created"; exit 1; }
CMD
expect_fail "negative: e2e GUI tests reference the user's display or a fixed display number" <<'CMD'
grep -nE -e 'DISPLAY=:[0-9]' -e 'Getenv\("DISPLAY"\)' -e 'LookupEnv\("DISPLAY"\)' e2e/xvfb_test.go e2e/gui_test.go
CMD
finish
