#!/usr/bin/env bash
# Verification for T035 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestRunGUIFinishSaves TestRunRawNoState TestRunResumeFile TestRunSignalSaves TestRunGUINoDisplay; do
expect_ok "$t passes" <<CMD
passes $t cmd/fastread
CMD
done
expect_ok "cmd tests pass with -tags nogui, CGO_ENABLED=0 and -race; real state dir untouched" 600 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
go test -tags nogui -count=1 ./cmd/fastread &&
CGO_ENABLED=0 go test -tags nogui -count=1 -run '^TestRun' ./cmd/fastread &&
go test -race -count=1 ./cmd/fastread || exit 1
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread"
CMD
expect_ok "runGUI seam and once-guard in run.go" <<'CMD'
grep -qE 'runGUI +=? *gui\.Run' cmd/fastread/run.go && grep -q 'sync.Once' cmd/fastread/run.go
CMD
expect_ok "GUI under Xvfb: SIGTERM saves the position, exit 0; raw text creates no state" 120 <<'CMD'
T="$(mktemp -d)"
go build -o "$T/fr" ./cmd/fastread || exit 1
for i in $(seq 0 99); do printf 'w%04d ' "$i"; done > "$T/book.txt"
exec 3>"$T/dpy"
Xvfb -displayfd 3 -screen 0 800x600x24 -nolisten tcp >"$T/xvfb.log" 2>&1 &
X=$!
exec 3>&-
trap 'kill $X 2>/dev/null; wait $X 2>/dev/null; rm -rf "$T"' EXIT
for i in $(seq 100); do test -s "$T/dpy" && break; sleep 0.1; done
D=":$(head -1 "$T/dpy")"; test "$D" != ":" || { echo "Xvfb did not start"; cat "$T/xvfb.log"; exit 1; }
prev=0
for round in 1 2; do
  env -u WAYLAND_DISPLAY DISPLAY="$D" XDG_STATE_HOME="$T/state" timeout --preserve-status -s TERM -k 3 3 \
    "$T/fr" --ui gui --wpm 50 "$T/book.txt" </dev/null 2>"$T/err"
  rc=$?; echo "round $round rc=$rc"; cat "$T/err"
  test "$rc" -eq 0 || exit 1
  set -- "$T/state/fastread"/*.json; test "$#" -eq 1 && test -f "$1" || { echo "no entry"; exit 1; }
  test "$(stat -c %a "$1")" = 600 || exit 1
  i="$(sed -n 's/.*"index": *\([0-9][0-9]*\).*/\1/p' "$1")"; echo "index=$i"
  test -n "$i" && test "$i" -gt "$prev" || exit 1
  prev="$i"
done
env -u WAYLAND_DISPLAY DISPLAY="$D" XDG_STATE_HOME="$T/state2" timeout --preserve-status -s TERM -k 3 2 \
  "$T/fr" --ui gui --wpm 50 "raw text only here" </dev/null 2>/dev/null; rc=$?
test "$rc" -eq 0 && test ! -e "$T/state2"
CMD
expect_ok "negative: no display with a file -> exit 1, one line, no state" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
printf 'one two three\n' > "$T/b.txt"
env -u DISPLAY -u WAYLAND_DISPLAY XDG_STATE_HOME="$T/state" "$T/fr" --ui gui "$T/b.txt" </dev/null >"$T/out" 2>"$T/err"; rc=$?
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && test ! -s "$T/out" && test ! -e "$T/state"
CMD
finish
