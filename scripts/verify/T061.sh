#!/usr/bin/env bash
# Verification for T061 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestGioMeasurer TestLayoutFocusX TestLayoutShrinkSplit TestLayoutFocusXAllSizes TestLayoutChrome TestGUIKeyMap TestRunNoDisplay TestWindowNoDisplay; do
expect_ok "$t passes" <<CMD
passes $t internal/gui
CMD
done
expect_ok "cmd GUI tests still pass (also -tags nogui); internal/gui -race" 900 <<'CMD'
passes TestRunGUIFinishSaves cmd/fastread && passes TestRunGUINoDisplay cmd/fastread &&
go test -tags nogui -count=1 ./cmd/fastread ./internal/gui && go test -race -count=1 ./internal/gui
CMD
expect_ok "window uses Layout, LayoutChrome, SetParts, InvalidateCmd at the deadline, the key map, goregular without system fonts" <<'CMD'
head -1 internal/gui/measure_gio.go | grep -qx '//go:build !nogui && cgo' &&
head -1 internal/gui/measure_gio_test.go | grep -qx '//go:build !nogui && cgo' &&
head -1 internal/gui/window.go | grep -qx '//go:build !nogui && cgo' || { echo "build tags"; exit 1; }
for s in 'Layout(' 'LayoutChrome(' 'SetParts(' 'InvalidateCmd' 'Deadline()' 'keyFilters()' 'keyAction(' 'op.Affine' 'ProgressText(' 'HelpText'; do
  grep -qF "$s" internal/gui/window.go || { echo "window.go lacks $s"; exit 1; }
done
cat internal/gui/window.go internal/gui/measure_gio.go | grep -q 'NoSystemFonts' &&
cat internal/gui/window.go internal/gui/measure_gio.go | grep -q 'gofont/goregular'
CMD
expect_ok "Xvfb pixels: 600x300 black window, red focus centred on W/2, ticks at W/2-1..W/2, ] [ ? p keys, resize, 1x1 survives, q exits 0" 300 <<'CMD'
. scripts/verify/_gui.sh; gui_setup || exit 1
words="$(for i in $(seq 60); do printf 'hello '; done)"
gui_start a --ui gui --no-resume --wpm 50 "$words" || exit 1
gui_frame f1 600 300 || exit 1
cat "$T/f1.txt"
test "$(pv f1 top)" -eq 0 || { echo "help line visible by default"; exit 1; }
test "$(pv f1 bottom)" -gt 0 || { echo "no progress at the bottom by default"; exit 1; }
r2="$(pv f1 red)"
gui_keys bracketright
gui_until 5 'gui_pix f2 && [ "$(pv f2 red)" -gt '"$r2"' ]' || { echo "] did not enlarge the focus letter"; cat "$T/f2.txt"; exit 1; }
gui_frame f2b 600 300 || exit 1
gui_keys bracketleft
gui_until 5 'gui_pix f3 && [ "$(pv f3 red)" -eq '"$r2"' ]' || { echo "[ did not restore size 2"; cat "$T/f3.txt"; exit 1; }
gui_keys question
gui_until 5 'gui_pix f4 && [ "$(pv f4 top)" -gt 0 ]' || { echo "? did not show the help line"; cat "$T/f4.txt"; exit 1; }
gui_frame f4b 600 300 || exit 1
gui_keys question p
gui_until 5 'gui_pix f5 && [ "$(pv f5 top)" -eq 0 ] && [ "$(pv f5 bottom)" -eq 0 ]' || { echo "? / p did not hide help and progress"; cat "$T/f5.txt"; exit 1; }
gui_frame f5b 600 300 || exit 1
gui_keys p
DISPLAY="$D" timeout 5 xdotool windowsize --sync "$WID" 801 401 >/dev/null 2>&1
gui_frame f6 801 401 || exit 1
DISPLAY="$D" timeout 5 xdotool windowsize "$WID" 1 1 >/dev/null 2>&1; sleep 1
kill -0 "$APP" 2>/dev/null || { echo "process died at 1x1"; cat "$T/a.err"; exit 1; }
DISPLAY="$D" timeout 5 xdotool windowsize --sync "$WID" 600 300 >/dev/null 2>&1
gui_frame f7 600 300 || exit 1
gui_keys q; gui_wait 3 || exit 1
test "$RC" -eq 0 || { echo "q: exit $RC"; cat "$T/a.err"; exit 1; }
test ! -s "$T/a.err" || { echo "unexpected stderr"; cat "$T/a.err"; exit 1; }
gui_start b --ui gui --no-progress --no-resume --wpm 50 "$words" || exit 1
gui_frame g1 600 300 || exit 1
test "$(pv g1 bottom)" -eq 0 || { echo "--no-progress still draws at the bottom"; cat "$T/g1.txt"; exit 1; }
gui_keys Escape; gui_wait 3 && test "$RC" -eq 0
CMD
expect_ok "Xvfb timing: words advance on the Player clock (SIGTERM after 3 s at 600 wpm -> index 15..40, exit 0)" 120 <<'CMD'
. scripts/verify/_gui.sh; gui_setup || exit 1
for i in $(seq 0 199); do printf 'w%03d ' "$i"; done > "$T/book.txt"
gui_start c --ui gui --no-resume --wpm 600 "$T/book.txt" || exit 1
sleep 3; kill -TERM "$APP"; gui_wait 3 || exit 1
test "$RC" -eq 0 || { echo "SIGTERM: exit $RC"; cat "$T/c.err"; exit 1; }
set -- "$T/state/fastread"/*.json; test -f "$1" || { echo "no state entry"; exit 1; }
i="$(sed -n 's/.*"index": *\([0-9][0-9]*\).*/\1/p' "$1")"; echo "index=$i"
test -n "$i" && test "$i" -ge 15 && test "$i" -le 40
CMD
expect_fail "negative: os.Exit or sleeping in internal/gui, Gio in untagged files" <<'CMD'
grep -n 'os\.Exit' internal/gui/*.go && exit 0
grep -nE 'time\.(Sleep|After)\(' internal/gui/window.go internal/gui/measure_gio.go && exit 0
for f in internal/gui/*.go; do
  case "$f" in *_test.go) continue;; esac
  head -1 "$f" | grep -qx '//go:build !nogui && cgo' && continue
  grep -l 'gioui.org' "$f" && exit 0
done
exit 1
CMD
finish
