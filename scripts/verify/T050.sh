#!/usr/bin/env bash
# Verification for T050 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestRestoreOnPanic TestLoopExitPaths TestLoopNoLeak TestLoopResize TestLoopKeys TestRunQuitRestores TestRunPanicRestores TestRunMakeRawFails TestRunNoGoroutineLeak; do
expect_ok "$t passes" <<CMD
passes $t internal/tui
CMD
done
expect_ok "exit-path, panic and leak tests pass under -race (3x) and -tags nogui" 900 <<'CMD'
go test -race -count=3 -run '^(TestRestoreOnPanic|TestLoopExitPaths|TestLoopNoLeak)$' ./internal/tui &&
go test -tags nogui -count=1 -run '^(TestRestoreOnPanic|TestLoopExitPaths|TestLoopNoLeak)$' ./internal/tui
CMD
expect_ok "all internal/tui and cmd tests still pass" 900 <<'CMD'
go test -count=1 ./internal/tui/... ./cmd/fastread
CMD
expect_fail "negative: os.Exit, .Fd(), re-panic or extra signal handling in internal/tui" <<'CMD'
grep -n -e '\.Fd()' -e 'os\.Exit' internal/tui/*.go && exit 0
ls internal/tui/*.go | grep -v '_test\.go$' | grep -v '/terminal\.go$' | xargs grep -n -e 'signal\.' -e 'panic(r)'
CMD
expect_ok "binary in tmux: q, Escape, C-c, SIGTERM, SIGINT restore screen, cursor and tty mode, exit 0; hang-up leaves no process" 240 <<'CMD'
T="$(mktemp -d)"; S="frv-t050-$$"
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
trap 'tmux -L "$S" kill-server 2>/dev/null; rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
words="$(for i in $(seq 0 29); do printf 'r%02d ' "$i"; done)"
fmt() { tmux -L "$S" display -p -t "$1" "$2" 2>/dev/null; }
start() { # $1 = session name
  rm -f "$T/rc" "$T/stty" "$T/err"
  tmux -L "$S" -f /dev/null new-session -d -s "$1" -x 80 -y 24 \
    "printf 'MAIN-MARK\n'; env XDG_STATE_HOME='$T/st' '$T/fr' --no-resume --size 2 --wpm 60 '$words' 2>'$T/err'; echo \$? > '$T/rc'; stty -a > '$T/stty'; sleep 30"
  for _ in $(seq 100); do test "$(fmt "$1" '#{alternate_on}:#{cursor_flag}')" = 1:0 && return 0; sleep 0.05; done
  echo "$1: never entered the alternate screen with hidden cursor"; return 1
}
for c in q Escape C-c sig:TERM sig:INT; do
  n="c$(printf '%s' "$c" | tr -c 'A-Za-z0-9' '_')"
  start "$n" || exit 1
  case "$c" in
    sig:*) pid="$(pgrep -f "^$T/fr ")"; test -n "$pid" || { echo "$c: no pid"; exit 1; }; kill -"${c#sig:}" "$pid" ;;
    *) tmux -L "$S" send-keys -t "$n" "$c" ;;
  esac
  for _ in $(seq 60); do test -s "$T/stty" && break; sleep 0.05; done
  test "$(cat "$T/rc" 2>/dev/null)" = 0 || { echo "$c: exit '$(cat "$T/rc" 2>/dev/null)'"; cat "$T/err"; exit 1; }
  st="$(fmt "$n" '#{alternate_on}:#{cursor_flag}')"
  test "$st" = 0:1 || { echo "$c: alternate_on:cursor_flag = $st, want 0:1"; exit 1; }
  tr ' ;' '\n\n' < "$T/stty" | grep -qx icanon || { echo "$c: tty not back in canonical mode"; cat "$T/stty"; exit 1; }
  tr ' ;' '\n\n' < "$T/stty" | grep -qx echo || { echo "$c: echo not restored"; exit 1; }
  tmux -L "$S" capture-pane -p -t "$n" | grep -q MAIN-MARK || { echo "$c: main screen content not back"; exit 1; }
  test ! -s "$T/err" || { echo "$c: unexpected stderr"; cat "$T/err"; exit 1; }
  tmux -L "$S" kill-session -t "$n"
done
start hup || exit 1
tmux -L "$S" kill-session -t hup
for _ in $(seq 40); do pgrep -f "^$T/fr " >/dev/null || break; sleep 0.05; done
! pgrep -f "^$T/fr " >/dev/null || { echo "fastread still running after the pane was killed"; pkill -f "^$T/fr "; exit 1; }
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || { echo "real state dir created"; exit 1; }
CMD
finish
