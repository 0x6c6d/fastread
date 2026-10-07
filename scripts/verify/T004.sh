#!/usr/bin/env bash
# Verification for T004 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestPlayerAdvance passes" <<'CMD'
passes TestPlayerAdvance internal/state
CMD
expect_ok "TestPlayerQuit passes" <<'CMD'
passes TestPlayerQuit internal/state
CMD
expect_ok "TestPlayerStartClamp passes" <<'CMD'
passes TestPlayerStartClamp internal/state
CMD
expect_ok "player tests pass under -race" <<'CMD'
go test -race -count=1 -run '^TestPlayer' ./internal/state
CMD
expect_ok "exported API present" <<'CMD'
d="$(go doc -all ./internal/state)" || exit 1
for s in 'func NewPlayer\(cfg Config\) \*Player' 'func \([a-z]+ \*Player\) Tick\(\) \(finished bool\)' \
  'func \([a-z]+ \*Player\) Apply\(a Action\) \(quit bool\)' 'func \([a-z]+ \*Player\) Deadline\(\) time.Time' \
  'func \([a-z]+ \*Player\) Token\(\) tokenize.Token' 'func \([a-z]+ \*Player\) Finished\(\) bool' \
  'type Clock interface' 'ActQuit' 'ActToggleHelp' 'ActHome'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
CMD
expect_ok "internal/state imports no UI package" <<'CMD'
out="$(go list -deps ./internal/state)" || exit 1
! printf '%s\n' "$out" | grep -q -e fastread/internal/tui -e fastread/internal/gui -e gioui.org
CMD
finish
