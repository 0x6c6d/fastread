#!/usr/bin/env bash
# Verification for T062 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestGUIExitPath TestGUIKeyMap TestGioMeasurer TestRunNoDisplay TestWindowNoDisplay TestLayoutShrinkSplit; do
expect_ok "$t passes" <<CMD
passes $t internal/gui
CMD
done
expect_ok "TestGUIExitPath under -race (3x), -tags nogui and CGO_ENABLED=0; cmd GUI tests" 900 <<'CMD'
go test -race -count=3 -run '^TestGUIExitPath$' ./internal/gui &&
go test -tags nogui -count=1 -v -run '^TestGUIExitPath$' ./internal/gui | grep -q -- '^--- PASS: TestGUIExitPath (' &&
CGO_ENABLED=0 go test -count=1 -v -run '^TestGUIExitPath$' ./internal/gui | grep -q -- '^--- PASS: TestGUIExitPath (' &&
passes TestRunGUIFinishSaves cmd/fastread && passes TestRunGUINoDisplay cmd/fastread &&
go test -tags nogui -count=1 ./cmd/fastread
CMD
expect_ok "session API declared in an untagged, Gio-free session.go; window.go uses it" <<'CMD'
test -f internal/gui/session.go && ! head -1 internal/gui/session.go | grep -q '^//go:build' || exit 1
for s in 'func newSession\(p \*state.Player, finish func\(last int, err error\)\) \*session' 'func \(s \*session\) end\(err error\)' \
  'func \(s \*session\) ended\(\) bool' 'func \(s \*session\) guard\(\)' 'func windowErr\(err error\) error'; do
  grep -qE "$s" internal/gui/session.go || { echo "missing: $s"; exit 1; }
done
grep -q 'newSession(' internal/gui/window.go && grep -q 'guard()' internal/gui/window.go && grep -q 'windowErr(' internal/gui/window.go
CMD
expect_ok "Xvfb: q, Escape, ctrl+c exit 0 (empty stderr); SIGINT/SIGTERM exit 0 and save; end of text exits 0" 300 <<'CMD'
. scripts/verify/_gui.sh; gui_setup || exit 1
words="$(for i in $(seq 60); do printf 'hello '; done)"
for k in q Escape ctrl+c; do
  gui_start "k$k" --ui gui --no-resume --wpm 60 "$words" || exit 1
  sleep 0.7; kill -0 "$APP" 2>/dev/null || { echo "$k: exited early"; cat "$T/k$k.err"; exit 1; }
  gui_keys "$k"; gui_wait 3 || exit 1
  test "$RC" -eq 0 || { echo "$k: exit $RC"; cat "$T/k$k.err"; exit 1; }
  test ! -s "$T/k$k.err" || { echo "$k: stderr"; cat "$T/k$k.err"; exit 1; }
done
for i in $(seq 0 99); do printf 'w%03d ' "$i"; done > "$T/book.txt"
for sig in INT TERM; do
  rm -rf "$T/state"
  gui_start "s$sig" --ui gui --no-resume --wpm 300 "$T/book.txt" || exit 1
  sleep 1.5; kill -"$sig" "$APP"; gui_wait 3 || exit 1
  test "$RC" -eq 0 || { echo "SIG$sig: exit $RC"; cat "$T/s$sig.err"; exit 1; }
  set -- "$T/state/fastread"/*.json; test -f "$1" || { echo "SIG$sig: no state entry"; exit 1; }
  test "$(stat -c %a "$1")" = 600 || exit 1
done
s=$(date +%s%N)
env -u WAYLAND_DISPLAY DISPLAY="$D" XDG_STATE_HOME="$T/st3" timeout -k 3 20 "$T/fr" --ui gui --wpm 1500 "a b c" </dev/null 2>"$T/end.err"; rc=$?
e=$(( ($(date +%s%N) - s) / 1000000 )); echo "end of text rc=$rc ms=$e"
test "$rc" -eq 0 && test "$e" -le 8000 && test ! -s "$T/end.err"
CMD
expect_ok "negative: unused DISPLAY and bogus WAYLAND_DISPLAY -> exit 1 within 10 s, one 'fastread: ' line, no panic" 120 <<'CMD'
. scripts/verify/_gui.sh
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
n="$(free_display)" || exit 1; mkdir -p "$T/run"; chmod 700 "$T/run"
for mode in x11 wayland; do
  if [ "$mode" = x11 ]; then
    env -u WAYLAND_DISPLAY DISPLAY=":$n" XDG_RUNTIME_DIR="$T/run" XDG_STATE_HOME="$T/st" timeout -k 2 10 "$T/fr" --ui gui "hello world" </dev/null >"$T/out" 2>"$T/err"; rc=$?
  else
    env -u DISPLAY WAYLAND_DISPLAY="fastread-verify-none-$$" XDG_RUNTIME_DIR="$T/run" XDG_STATE_HOME="$T/st" timeout -k 2 10 "$T/fr" --ui gui "hello world" </dev/null >"$T/out" 2>"$T/err"; rc=$?
  fi
  echo "$mode rc=$rc: $(cat "$T/err")"
  test "$rc" -eq 1 || exit 1
  test "$(wc -l < "$T/err")" -eq 1 && grep -q '^fastread: ' "$T/err" || exit 1
  ! grep -q -e 'panic' -e 'goroutine ' "$T/err" || exit 1
  test ! -s "$T/out" || exit 1
done
CMD
expect_ok "negative: no display at all with a file -> exit 1, one line, no state" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
printf 'one two three\n' > "$T/b.txt"
env -u DISPLAY -u WAYLAND_DISPLAY XDG_STATE_HOME="$T/state" timeout 10 "$T/fr" --ui gui "$T/b.txt" </dev/null >"$T/out" 2>"$T/err"; rc=$?
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && test ! -s "$T/out" && test ! -e "$T/state"
CMD
expect_fail "negative: os.Exit in internal/gui or Gio in session.go/gui.go/stub.go/layout.go/chrome.go" <<'CMD'
grep -n 'os\.Exit' internal/gui/*.go && exit 0
grep -n 'gioui.org' internal/gui/session.go internal/gui/gui.go internal/gui/stub.go internal/gui/layout.go internal/gui/chrome.go
CMD
finish
