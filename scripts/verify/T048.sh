#!/usr/bin/env bash
# Verification for T048 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestLoopKeys TestLoopSplitSteps TestLoopDriftRealClock TestKeyDecode TestRunQuitRestores TestRunPlaysToEnd TestRunCtxCancel TestRunPanicRestores TestRunMakeRawFails TestRunNoGoroutineLeak; do
expect_ok "$t passes" <<CMD
passes $t internal/tui
CMD
done
expect_ok "loop and run tests pass under -race (2x) and -tags nogui" 900 <<'CMD'
go test -race -count=2 -run '^(TestLoop|TestRun|TestKeyDecode$)' ./internal/tui &&
go test -tags nogui -count=1 -run '^(TestLoop|TestRun)' ./internal/tui
CMD
expect_ok "cmd tests (resume, signal save, TUI wiring) still pass" 600 <<'CMD'
for t in TestRunSignalSaves TestRunResumeFile TestRunRawNoState TestRunTUIPlaysToEnd; do passes $t cmd/fastread || exit 1; done
go test -race -count=1 ./cmd/fastread
CMD
expect_ok "loop.go holds Run; seams declared; Run signature unchanged" <<'CMD'
test -f internal/tui/loop.go && test -f internal/tui/keys.go && test -f internal/tui/terminal.go || exit 1
grep -qE '^func Run\(ctx context\.Context, t Terminal, p \*state\.Player, opts Options\) \(last int, err error\)' internal/tui/loop.go &&
grep -qE 'renderFn +=? *Render' internal/tui/*.go && grep -qE 'encodeFn +=? *Encode' internal/tui/*.go &&
grep -qE 'frameHook +func\((m )?Model, (f )?Frame\)' internal/tui/*.go &&
grep -q 'SetParts(' internal/tui/loop.go && grep -q 'Split(' internal/tui/loop.go
CMD
expect_fail "negative: time.Sleep or time.After in loop.go" <<'CMD'
grep -nE -e 'time\.Sleep' -e 'time\.After\(' internal/tui/loop.go
CMD
expect_fail "negative: .Fd(), os.Exit or Gio in internal/tui" <<'CMD'
grep -n -e '\.Fd()' -e 'os\.Exit' -e 'gioui.org' internal/tui/*.go && exit 0
out="$(go list -deps ./internal/tui)" || exit 0
printf '%s\n' "$out" | grep -q -e fastread/internal/gui -e gioui.org
CMD
expect_ok "binary in tmux: real key sequences work, unknown keys ignored (negative), Esc quits" 180 <<'CMD'
export LC_ALL=C.UTF-8
T="$(mktemp -d)"; S="frv-t048-$$"
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
trap 'tmux -L "$S" kill-server 2>/dev/null; rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
words="$(for i in $(seq 0 59); do printf 'k%03d ' "$i"; done)"
tmux -L "$S" -f /dev/null new-session -d -s v -x 80 -y 24 \
  "env XDG_STATE_HOME='$T/st' '$T/fr' --no-resume --size 1 --wpm 50 '$words' 2>'$T/err'; echo \$? > '$T/rc'"
word() { tmux -L "$S" capture-pane -p -t v 2>/dev/null | grep -o 'k[0-9][0-9][0-9]' | head -1; }
waitword() { for _ in $(seq 40); do test "$(word)" = "$1" && return 0; sleep 0.05; done; echo "want $1, screen shows '$(word)'"; return 1; }
keys() { tmux -L "$S" send-keys -t v "$@"; }
alive() { test ! -e "$T/rc"; }
for _ in $(seq 100); do test -n "$(word)" && break; sleep 0.05; done
keys Space; sleep 0.3; n="$(word)"; test -n "$n" || { echo "no word on screen"; exit 1; }
i=$((10#${n#k}))
keys PPage F1 x Q; sleep 0.5
alive || { echo "unknown keys quit the program"; cat "$T/err"; exit 1; }
test "$(word)" = "$n" || { echo "unknown keys changed the word"; exit 1; }
keys Right; waitword "$(printf 'k%03d' $((i + 1)))" || exit 1
keys Left; waitword "$n" || exit 1
keys Up Down; sleep 0.3
alive && test "$(word)" = "$n" || { echo "Up/Down quit or moved the word"; exit 1; }
keys Home; waitword k000 || exit 1
keys Space Right
for _ in $(seq 40); do w="$(word)"; test "${w:-k000}" \> k009 && break; sleep 0.05; done
test "$(word)" \> k009 || { echo "Right while playing did not jump 10"; exit 1; }
keys Escape
for _ in $(seq 40); do test -s "$T/rc" && break; sleep 0.05; done
test "$(cat "$T/rc" 2>/dev/null)" = 0 || { echo "Esc did not exit 0"; cat "$T/err"; exit 1; }
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || { echo "real state dir created"; exit 1; }
CMD
finish
