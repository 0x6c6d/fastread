#!/usr/bin/env bash
# Verification for T049 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestLoopResize TestTTYPty TestLoopKeys TestLoopSplitSteps TestLoopDriftRealClock TestRunQuitRestores TestRunCtxCancel TestRunPanicRestores TestRunNoGoroutineLeak; do
expect_ok "$t passes" <<CMD
passes $t internal/tui
CMD
done
expect_ok "resize and pty tests pass under -race (3x) and -tags nogui; CGO_ENABLED=0 build" 900 <<'CMD'
go test -race -count=3 -run '^(TestLoopResize|TestTTYPty)$' ./internal/tui &&
go test -tags nogui -count=1 -run '^(TestLoopResize|TestTTYPty)$' ./internal/tui &&
CGO_ENABLED=0 go test -tags nogui -count=1 -run '^TestTTYPty$' ./internal/tui
CMD
expect_ok "cmd tests still pass" 600 <<'CMD'
go test -count=1 ./cmd/fastread
CMD
expect_ok "Resizer, newTTY and SIGWINCH registration/release declared" <<'CMD'
d="$(go doc -all ./internal/tui)" || exit 1
printf '%s\n' "$d" | grep -qE 'type Resizer interface' && printf '%s\n' "$d" | grep -qE 'Resized\(\) <-chan struct\{\}' || { echo "Resizer missing"; exit 1; }
grep -qE 'func newTTY\(f \*os\.File\) \(\*[A-Za-z_][A-Za-z0-9_]*, error\)' internal/tui/terminal.go &&
grep -q 'signal.Notify(' internal/tui/terminal.go && grep -q 'syscall.SIGWINCH\|unix.SIGWINCH' internal/tui/terminal.go &&
grep -q 'signal.Stop(' internal/tui/terminal.go && grep -q 'Resizer' internal/tui/loop.go
CMD
expect_fail "negative: .Fd() or os.Exit in internal/tui, or signal handling outside terminal.go" <<'CMD'
grep -n -e '\.Fd()' -e 'os\.Exit' internal/tui/*.go && exit 0
ls internal/tui/*.go | grep -v '_test\.go$' | grep -v '/terminal\.go$' | xargs grep -n 'signal\.' 
CMD
expect_ok "binary in tmux: live resize 80x24 -> 40x8 -> 19x4 -> 1x1 -> 10x3 -> 80x24, no crash, q exits 0" 180 <<'CMD'
export LC_ALL=C.UTF-8
T="$(mktemp -d)"; S="frv-t049-$$"
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
trap 'tmux -L "$S" kill-server 2>/dev/null; rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
words="$(for i in $(seq 40); do printf 'wonderful '; done)"
tmux -L "$S" -f /dev/null new-session -d -s v -x 80 -y 24 \
  "env XDG_STATE_HOME='$T/st' COLORTERM=truecolor '$T/fr' --no-resume --size 3 --wpm 60 '$words' 2>'$T/err'; echo \$? > '$T/rc'"
scr() { tmux -L "$S" capture-pane -p -t v 2>/dev/null; }
ticks() { scr | { n=0; c=""; while IFS= read -r l; do case "$l" in *│*) p="${l%%│*}"; n=$((n + 1)); c="$c ${#p}";; esac; done; echo "$n:${c# }"; }; }
waitticks() { for _ in $(seq 60); do test "$(ticks)" = "$1" && return 0; sleep 0.05; done; echo "want ticks $1, got $(ticks)"; scr; return 1; }
alive() { test ! -e "$T/rc"; }
rs() { tmux -L "$S" resize-window -t v -x "$1" -y "$2"; }
waitticks '2:40 40' || exit 1
tmux -L "$S" send-keys -t v Space
rs 40 8; waitticks '2:20 20' || exit 1
rs 19 4; ok=0
for _ in $(seq 60); do test "$(scr | grep -v '^ *$' | sed 's/^ *//; s/ *$//')" = 'terminal too small' && { ok=1; break; }; sleep 0.05; done
test "$ok" = 1 || { echo "19x4 did not show only 'terminal too small'"; scr; exit 1; }
rs 1 1; sleep 0.3; rs 10 3; sleep 0.5
alive || { echo "process exited on a tiny size: rc $(cat "$T/rc")"; cat "$T/err"; exit 1; }
rs 80 24; waitticks '2:40 40' || exit 1
rs 121 30; waitticks '2:60 60' || exit 1
tmux -L "$S" send-keys -t v q
for _ in $(seq 40); do test -s "$T/rc" && break; sleep 0.05; done
test "$(cat "$T/rc" 2>/dev/null)" = 0 || { echo "q did not exit 0"; cat "$T/err"; exit 1; }
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || { echo "real state dir created"; exit 1; }
CMD
finish
