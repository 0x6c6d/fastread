#!/usr/bin/env bash
# Verification for T056 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Also the Phase 5 exit criterion of Plan.md.
. "$(dirname "$0")/_lib.sh"
expect_ok "Phase 5 exit criterion (verbatim)" 1800 <<'CMD'
check && for t in TestKeyDecode TestRestoreOnPanic TestLoopNoLeak TestLoopDriftRealClock TestLoopResize; do passes $t internal/tui || exit 1; done && go test -tags e2e -count=1 -timeout 300s -run '^TestE2E(Basic|Stdin|NotFound|NoArgTTY|Restore|FocusColumn|Resize|SIGTERMSavesState|Resume)$' ./e2e/
CMD
expect_ok "e2e files carry the e2e build tag, are test files, vet clean; keys_test.go exists" <<'CMD'
test -f e2e/keys_test.go || exit 1
for f in e2e/*.go; do
  case "$f" in *_test.go) ;; *) echo "$f is not a _test.go file"; exit 1;; esac
  head -1 "$f" | grep -qx '//go:build e2e' || { echo "$f lacks //go:build e2e"; exit 1; }
done
go vet -tags e2e ./e2e/
CMD
expect_ok "every Phase 5 e2e test and TestE2EKeys exists and passes (-v); no leftovers; real state dir untouched" 600 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
out="$(go test -tags e2e -count=1 -timeout 420s -v -run '^TestE2E(Basic|Stdin|NotFound|NoArgTTY|Restore|FocusColumn|Resize|SIGTERMSavesState|Resume|Keys)$' ./e2e/ 2>&1)"; rc=$?
printf '%s\n' "$out" | tail -n 60
test "$rc" -eq 0 || exit 1
for n in TestE2EBasic TestE2EStdin TestE2ENotFound TestE2ENoArgTTY TestE2ERestore TestE2EFocusColumn TestE2EResize TestE2ESIGTERMSavesState TestE2EResume TestE2EKeys; do
  printf '%s\n' "$out" | grep -qE -- "^--- PASS: $n \(" || { echo "no PASS line for $n"; exit 1; }
done
sleep 0.5
if ls "/tmp/tmux-$(id -u)" 2>/dev/null | grep -q '^fastread-e2e-'; then
  for s in $(ls "/tmp/tmux-$(id -u)" | grep '^fastread-e2e-'); do
    tmux -L "$s" ls >/dev/null 2>&1 && { echo "tmux server $s still running"; exit 1; }
  done
fi
! pgrep -u "$(id -u)" -x fastread >/dev/null || { echo "fastread process left running"; exit 1; }
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || { echo "real state dir created"; exit 1; }
CMD
expect_ok "D7: Phase 5 internal/tui tests also pass with -race and -tags nogui" 900 <<'CMD'
r='^(TestKeyDecode|TestRestoreOnPanic|TestLoopNoLeak|TestLoopDriftRealClock|TestLoopResize|TestLoopKeys|TestLoopSplitSteps|TestLoopExitPaths|TestTTYPty)$'
go test -race -count=1 -run "$r" ./internal/tui && go test -tags nogui -count=1 -run "$r" ./internal/tui
CMD
expect_ok "Phase 5 state tests still pass" <<'CMD'
passes TestPlayerSteps internal/state && passes TestPlayerStepsNoDrift internal/state
CMD
expect_ok "protected files unchanged" <<'CMD'
scripts/check-protected.sh
CMD
expect_fail "negative: internal/tui imports Gio, internal/gui or network packages" <<'CMD'
out="$(go list -deps ./internal/tui ./internal/tui/glyph)" || exit 0
printf '%s\n' "$out" | grep -q -x -e 'net' -e 'net/http' -e 'github.com/0x6c6d/fastread/internal/gui' -e 'gioui.org.*'
CMD
expect_fail "negative: logic packages import a UI package" <<'CMD'
out="$(go list -deps ./internal/input ./internal/tokenize ./internal/orp ./internal/timing ./internal/state)" || exit 0
printf '%s\n' "$out" | grep -q -e 'fastread/internal/tui' -e 'fastread/internal/gui' -e 'gioui.org'
CMD
finish
