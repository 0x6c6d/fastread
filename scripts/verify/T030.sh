#!/usr/bin/env bash
# Verification for T030 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestPlayerKeys TestPlayerQuit TestPlayerAdvance TestPlayerStartClamp; do
expect_ok "$t passes" <<CMD
passes $t internal/state
CMD
done
expect_ok "player tests pass under -race" <<'CMD'
go test -race -count=1 -run '^TestPlayer' ./internal/state
CMD
expect_ok "TODO marker removed, Apply signature unchanged" <<'CMD'
! grep -q 'TODO(phase3)' internal/state/player.go &&
go doc -all ./internal/state | grep -qE 'func \([a-z]+ \*Player\) Apply\(a Action\) \(quit bool\)'
CMD
expect_fail "negative: player.go does I/O, sleeps or starts goroutines" <<'CMD'
grep -nE -e '"os"' -e '"io"' -e 'time\.Sleep' -e 'time\.Now\(' -e '\bgo func' internal/state/player.go
CMD
expect_fail "negative: internal/state imports a UI package or Gio" <<'CMD'
out="$(go list -deps ./internal/state)" || exit 0
printf '%s\n' "$out" | grep -q -e fastread/internal/tui -e fastread/internal/gui -e gioui.org
CMD
finish
