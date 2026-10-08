#!/usr/bin/env bash
# Verification for T031 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestEffectiveWPM TestScheduleNoDrift TestPlayerKeys TestPlayerQuit TestPlayerAdvance TestPlayerStartClamp; do
expect_ok "$t passes" <<CMD
passes $t internal/state
CMD
done
expect_ok "player tests pass under -race and repeated" <<'CMD'
go test -race -count=1 -run '^(TestPlayer|TestEffectiveWPM$|TestScheduleNoDrift$)' ./internal/state &&
go test -count=5 -run '^(TestPlayer|TestEffectiveWPM$|TestScheduleNoDrift$)' ./internal/state
CMD
expect_ok "EffectiveWPM signature" <<'CMD'
go doc -all ./internal/state | grep -qE 'func \([a-z]+ \*Player\) EffectiveWPM\(\) \(wpm int, ok bool\)'
CMD
expect_fail "negative: player.go uses the real clock, sleeps or goroutines" <<'CMD'
grep -nE -e 'time\.Sleep' -e 'time\.Now\(' -e 'time\.Since\(' -e '\bgo func' internal/state/player.go
CMD
finish
