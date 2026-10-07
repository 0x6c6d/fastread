#!/usr/bin/env bash
# Verification for T010 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestRunEmptyText TestRunNoInputTTY TestRunStartBeyond TestRunTerminalError TestRunTUIPlaysToEnd TestRunGUINoDisplay; do
expect_ok "$t passes" <<CMD
passes $t cmd/fastread
CMD
done
expect_ok "cmd tests pass with -tags nogui" <<'CMD'
go test -tags nogui -count=1 -run '^TestRun' ./cmd/fastread
CMD
expect_ok "layering (R30): logic packages import no UI, gioui.org only in internal/gui" <<'CMD'
out="$(go list -deps ./internal/input ./internal/tokenize ./internal/orp ./internal/timing ./internal/state)" || exit 1
test -z "$(printf '%s\n' "$out" | grep -e fastread/internal/tui -e fastread/internal/gui -e gioui.org)" &&
test -z "$(grep -rl gioui.org --include='*.go' . | grep -v '^./internal/gui/')"
CMD
expect_ok "GUI under Xvfb stays up >= 2.5 s and exits 0 within 1.5 s of SIGTERM" 90 <<'CMD'
T="$(mktemp -d)"
go build -o "$T/fr" ./cmd/fastread || exit 1
exec 3>"$T/dpy"
Xvfb -displayfd 3 -screen 0 800x600x24 -nolisten tcp >"$T/xvfb.log" 2>&1 &
X=$!
exec 3>&-
trap 'kill $X 2>/dev/null; wait $X 2>/dev/null; rm -rf "$T"' EXIT
for i in $(seq 100); do test -s "$T/dpy" && break; sleep 0.1; done
D=":$(head -1 "$T/dpy")"; test "$D" != ":" || { echo "Xvfb did not start"; cat "$T/xvfb.log"; exit 1; }
s=$(date +%s%N)
env -u WAYLAND_DISPLAY DISPLAY="$D" XDG_STATE_HOME="$T/state" timeout --preserve-status -k 3 3 \
  "$T/fr" --ui gui --wpm 50 "one two three four five six seven eight" </dev/null 2>"$T/err"
rc=$?; e=$(( ($(date +%s%N) - s) / 1000000 ))
echo "rc=$rc elapsed=${e}ms"; cat "$T/err"
test "$rc" -eq 0 && test "$e" -ge 2500 && test "$e" -le 4500
CMD
expect_ok "negative: no display, cgo binary: exit 1, one stderr line" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fg" ./cmd/fastread || exit 1
env -u DISPLAY -u WAYLAND_DISPLAY XDG_STATE_HOME="$T/state" "$T/fg" --ui gui x </dev/null >"$T/out" 2>"$T/err"; rc=$?
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && grep -q '^fastread: ' "$T/err" && test ! -s "$T/out"
CMD
expect_ok "negative: nogui CGO_ENABLED=0 binary: no gioui.org, --ui gui exits 1 with one line" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
CGO_ENABLED=0 go build -tags nogui -o "$T/fr" ./cmd/fastread || exit 1
! go version -m "$T/fr" | grep -q gioui.org || exit 1
! go tool nm "$T/fr" | grep -q gioui.org || exit 1
for d in unset :0; do
  if [ "$d" = unset ]; then env -u DISPLAY -u WAYLAND_DISPLAY "$T/fr" --ui gui x </dev/null 2>"$T/err"; rc=$?
  else env DISPLAY="$d" "$T/fr" --ui gui x </dev/null 2>"$T/err"; rc=$?; fi
  test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 || { echo "DISPLAY=$d rc=$rc"; cat "$T/err"; exit 1; }
done
CMD
expect_ok "negative: empty raw text and empty stdin exit 1 with one line" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
"$T/fr" "   " </dev/null 2>"$T/e1"; r1=$?
printf '' | "$T/fr" 2>"$T/e2"; r2=$?
test "$r1" -eq 1 && test "$r2" -eq 1 && test "$(wc -l < "$T/e1")" -eq 1 && test "$(wc -l < "$T/e2")" -eq 1 &&
grep -q 'empty' "$T/e1" && grep -q 'empty' "$T/e2"
CMD
expect_ok "negative: no controlling terminal -> exit 1, one line, nothing on stdout" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
setsid -w "$T/fr" --no-resume --wpm 1500 "hello world" </dev/null >"$T/out" 2>"$T/err"; rc=$?
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && test ! -s "$T/out"
CMD
expect_ok "negative: no argument with stdin on a TTY -> exit 2 with usage" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
script -qec "$T/fr" /dev/null </dev/null >"$T/out" 2>&1; rc=$?
test "$rc" -eq 2 && grep -q 'Usage:' "$T/out"
CMD
finish
