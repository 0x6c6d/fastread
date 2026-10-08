#!/usr/bin/env bash
# Verification for T046 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestPlayerSteps TestPlayerStepsNoDrift TestPlayerKeys TestPlayerQuit TestPlayerAdvance TestPlayerStartClamp TestEffectiveWPM TestScheduleNoDrift; do
expect_ok "$t passes" <<CMD
passes $t internal/state
CMD
done
expect_ok "player and step tests pass under -race, repeated 3x" 600 <<'CMD'
go test -race -count=3 -run '^(TestPlayer|TestEffectiveWPM$|TestScheduleNoDrift$)' ./internal/state
CMD
expect_ok "SetParts, Part, Parts declared; steps_test.go exists" <<'CMD'
test -f internal/state/steps_test.go || exit 1
d="$(go doc -all ./internal/state)" || exit 1
for s in 'func \(p \*Player\) SetParts\(parts \[\]string\)' 'func \(p \*Player\) Part\(\) int' 'func \(p \*Player\) Parts\(\) int'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
CMD
expect_fail "negative: real clock, sleep, goroutine or I/O in player.go" <<'CMD'
grep -nE -e 'time\.Now\(' -e 'time\.Sleep' -e 'time\.After' -e '^\s*go ' -e '"os"' internal/state/player.go
CMD
expect_fail "negative: internal/state imports a UI package" <<'CMD'
out="$(go list -deps ./internal/state)" || exit 0
printf '%s\n' "$out" | grep -q -e 'fastread/internal/tui' -e 'fastread/internal/gui' -e 'gioui.org'
CMD
finish
