#!/usr/bin/env bash
# Verification for T065 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Also the Phase 6 exit criterion of Plan.md.
. "$(dirname "$0")/_lib.sh"
expect_ok "Phase 6 exit criterion (verbatim)" 1800 <<'CMD'
check && for t in TestLayoutFocusX TestLayoutBaseline TestLayoutTicks TestLayoutShrinkSplit TestLayoutFocusXAllSizes TestGUIKeyMap; do passes $t internal/gui || exit 1; done && go test -tags e2e -count=1 -timeout 300s -run '^TestE2E(GUIXvfb|GUINoDisplay|GUIBadDisplay|NoguiBinary)$' ./e2e/
CMD
expect_ok "e2e files tagged, vet clean; guikeys_test.go exists" <<'CMD'
test -f e2e/guikeys_test.go || exit 1
for f in e2e/*.go; do
  case "$f" in *_test.go) ;; *) echo "$f is not a _test.go file"; exit 1;; esac
  head -1 "$f" | grep -qx '//go:build e2e' || { echo "$f lacks //go:build e2e"; exit 1; }
done
go vet -tags e2e ./e2e/
CMD
expect_ok "every Phase 6 e2e test and TestE2EGUIKeys pass (-v); nothing left running; real state dir untouched" 900 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
xv0="$(pgrep -u "$(id -u)" -x Xvfb | sort)"
out="$(go test -tags e2e -count=1 -timeout 600s -v -run '^TestE2E(GUIXvfb|GUINoDisplay|GUIBadDisplay|NoguiBinary|GUIKeys|Basic)$' ./e2e/ 2>&1)"; rc=$?
printf '%s\n' "$out" | tail -n 60
test "$rc" -eq 0 || exit 1
for n in TestE2EGUIXvfb TestE2EGUINoDisplay TestE2EGUIBadDisplay TestE2ENoguiBinary TestE2EGUIKeys TestE2EBasic; do
  printf '%s\n' "$out" | grep -qE -- "^--- PASS: $n \(" || { echo "no PASS line for $n"; exit 1; }
done
sleep 0.5
test "$(pgrep -u "$(id -u)" -x Xvfb | sort)" = "$xv0" || { echo "an Xvfb started by the tests is still running"; exit 1; }
! pgrep -u "$(id -u)" -x fastread >/dev/null || { echo "fastread process left running"; exit 1; }
if ls "/tmp/tmux-$(id -u)" 2>/dev/null | grep -q '^fastread-e2e-'; then
  for s in $(ls "/tmp/tmux-$(id -u)" | grep '^fastread-e2e-'); do
    tmux -L "$s" ls >/dev/null 2>&1 && { echo "tmux server $s still running"; exit 1; }
  done
fi
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || { echo "real state dir created"; exit 1; }
CMD
expect_ok "D8: Phase 6 internal/gui tests under -race; layout and exit-path tests with -tags nogui; AllSizes coverage" 900 <<'CMD'
go test -race -count=1 -run '^(TestLayoutFocusX|TestLayoutBaseline|TestLayoutTicks|TestLayoutShrinkSplit|TestLayoutFocusXAllSizes|TestLayoutChrome|TestGUIKeyMap|TestGUIExitPath|TestGioMeasurer)$' ./internal/gui || exit 1
go test -tags nogui -count=1 -run '^(TestLayout|TestGUIExitPath$)' ./internal/gui || exit 1
out="$(go test -count=1 -v -run '^TestLayoutFocusXAllSizes$' ./internal/gui 2>&1)" || exit 1
line="$(printf '%s\n' "$out" | grep -oE 'words=[0-9]+ levels=5 widths=200,600,601,1920 cases=[0-9]+' | head -1)"
echo "$line"; test -n "$line" && test "$(printf '%s\n' "$line" | sed 's/^words=\([0-9]*\).*/\1/')" -ge 30
CMD
expect_ok "D10 layering (R30) verbatim" <<'CMD'
test -z "$(go list -deps ./internal/input ./internal/tokenize ./internal/orp ./internal/timing ./internal/state | grep -e fastread/internal/tui -e fastread/internal/gui -e gioui.org)" && test -z "$(grep -rl gioui.org --include='*.go' . | grep -v '^./internal/gui/')"
CMD
expect_ok "D11 (nogui binary) verbatim" 300 <<'CMD'
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
CGO_ENABLED=0 go build -tags nogui -o $T/fr ./cmd/fastread && ! go version -m $T/fr | grep -q gioui.org && ! go tool nm $T/fr | grep -q gioui.org && { env -u DISPLAY -u WAYLAND_DISPLAY $T/fr --ui gui x 2>$T/err; test $? -eq 1; } && test "$(wc -l < $T/err)" -eq 1
CMD
expect_ok "D12 (no-display GUI) verbatim" 300 <<'CMD'
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
go build -o $T/fg ./cmd/fastread && { env -u DISPLAY -u WAYLAND_DISPLAY $T/fg --ui gui x 2>$T/err2; test $? -eq 1; } && test "$(wc -l < $T/err2)" -eq 1
CMD
expect_ok "D13 protected files unchanged" <<'CMD'
scripts/check-protected.sh
CMD
expect_ok "live pixels on Xvfb: centred focus, ticks, ] / ? / p, resize 1280x720, q exits 0" 300 <<'CMD'
. scripts/verify/_gui.sh; gui_setup || exit 1
words="$(for i in $(seq 60); do printf 'hello '; done)"
gui_start a --ui gui --no-resume --wpm 50 "$words" || exit 1
gui_frame p1 600 300 || exit 1
r="$(pv p1 red)"
gui_keys bracketright bracketright bracketright
gui_until 5 'gui_pix p2 && [ "$(pv p2 red)" -gt '"$r"' ]' || { echo "] did not enlarge"; exit 1; }
gui_frame p2b 600 300 10 || exit 1
gui_keys question p
gui_until 5 'gui_pix p3 && [ "$(pv p3 top)" -gt 0 ] && [ "$(pv p3 bottom)" -eq 0 ]' || { echo "? / p toggles"; cat "$T/p3.txt"; exit 1; }
gui_frame p3b 600 300 10 || exit 1
DISPLAY="$D" timeout 5 xdotool windowsize --sync "$WID" 1280 720 >/dev/null 2>&1
gui_frame p4 1280 720 10 || exit 1
gui_keys q; gui_wait 3 && test "$RC" -eq 0 && test ! -s "$T/a.err"
CMD
expect_fail "negative: untagged internal/gui files import Gio, or the nogui build depends on Gio" <<'CMD'
for f in internal/gui/*.go; do
  head -1 "$f" | grep -qx '//go:build !nogui && cgo' && continue
  grep -l 'gioui.org' "$f" && exit 0
done
out="$(go list -tags nogui -deps ./cmd/fastread ./internal/gui)" || exit 0
printf '%s\n' "$out" | grep -q '^gioui.org'
CMD
expect_fail "negative: logic packages import a UI package" <<'CMD'
out="$(go list -deps ./internal/input ./internal/tokenize ./internal/orp ./internal/timing ./internal/state)" || exit 0
printf '%s\n' "$out" | grep -q -e 'fastread/internal/tui' -e 'fastread/internal/gui' -e 'gioui.org'
CMD
finish
