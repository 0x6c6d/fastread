#!/usr/bin/env bash
# Verification for T041 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestSplitLongWord TestSplitLinear TestFocusColumn TestWideAndCombining TestTicks TestTooSmall TestSizeFallback TestGlyphFallback TestRenderFocusLevel1 TestRenderClip; do
expect_ok "$t passes" <<CMD
passes $t internal/tui
CMD
done
expect_ok "split and focus tests pass under -race" 600 <<'CMD'
go test -race -count=1 -run '^(TestSplitLongWord|TestSplitLinear|TestFocusColumn)$' ./internal/tui
CMD
expect_ok "Split signature, Part and Parts fields" <<'CMD'
go doc -all ./internal/tui | grep -qE 'func Split\(word string, size, w int\) \[\]string' &&
go doc ./internal/tui Model | grep -qE '^[[:space:]]+Part +int' &&
go doc ./internal/tui Frame | grep -qE '^[[:space:]]+Parts +int'
CMD
expect_ok "race constant files" <<'CMD'
head -1 internal/tui/race_on_test.go | grep -qxF '//go:build race' &&
head -1 internal/tui/race_off_test.go | grep -qxF '//go:build !race' &&
grep -q 'raceEnabled' internal/tui/race_on_test.go internal/tui/race_off_test.go
CMD
expect_ok "TestFocusColumn covers the AC13 word classes" <<'CMD'
f=internal/tui/split_test.go; test -f "$f" || exit 1
for w in 'naïve' '日本語の本' '👍' 'extraordinary' 'Donaudampfschifffahrtsgesellschaftskapitän' '1,000,000' '121'; do
  grep -qF -- "$w" "$f" || { echo "missing: $w"; exit 1; }
done
CMD
finish
