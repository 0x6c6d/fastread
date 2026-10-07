#!/usr/bin/env bash
# Verification for T007 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestRunQuitRestores passes" <<'CMD'
passes TestRunQuitRestores internal/tui
CMD
expect_ok "TestRunPlaysToEnd passes" <<'CMD'
passes TestRunPlaysToEnd internal/tui
CMD
expect_ok "TestRunCtxCancel passes" <<'CMD'
passes TestRunCtxCancel internal/tui
CMD
expect_ok "negative: TestRunPanicRestores passes (panic recovered, terminal restored)" <<'CMD'
passes TestRunPanicRestores internal/tui
CMD
expect_ok "negative: TestRunMakeRawFails passes (nothing written on MakeRaw error)" <<'CMD'
passes TestRunMakeRawFails internal/tui
CMD
expect_ok "TestRunNoGoroutineLeak passes" <<'CMD'
passes TestRunNoGoroutineLeak internal/tui
CMD
expect_ok "Run tests pass under -race, repeated 3x" 600 <<'CMD'
go test -race -count=3 -run '^TestRun' ./internal/tui
CMD
expect_ok "pins x/term v0.40.0 and x/sys v0.41.0, go 1.24.x" <<'CMD'
grep -qE '^[[:space:]]*(require[[:space:]]+)?golang.org/x/term v0\.40\.0$' go.mod &&
grep -qE '^[[:space:]]*(require[[:space:]]+)?golang.org/x/sys v0\.41\.0( // indirect)?$' go.mod &&
grep -qE '^go 1\.24(\.[0-9]+)?$' go.mod && GOTOOLCHAIN=local go build ./... && go mod verify
CMD
expect_ok "exported API present" <<'CMD'
d="$(go doc -all ./internal/tui)" || exit 1
for s in 'func Run\(ctx context.Context, t Terminal, p \*state.Player, opts Options\) \(last int, err error\)' \
  'func OpenTTY\(\) \(Terminal, error\)' 'type Terminal interface' 'EnterSeq' 'RestoreSeq' \
  'MakeRaw\(\) error' 'Restore\(\) error' 'Size\(\) \(w, h int, err error\)'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
CMD
expect_fail "negative: .Fd() or os.Exit used in internal/tui" <<'CMD'
ls internal/tui/*.go >/dev/null 2>&1 || exit 0
grep -n -e '\.Fd()' -e 'os\.Exit' internal/tui/*.go
CMD
expect_fail "negative: internal/tui imports Gio or internal/gui" <<'CMD'
out="$(go list -deps ./internal/tui)" || exit 0
printf '%s\n' "$out" | grep -q -e fastread/internal/gui -e gioui.org
CMD
finish
