#!/usr/bin/env bash
# Verification for T001 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "module path and go 1.24.x directive" <<'CMD'
grep -qx 'module github.com/0x6c6d/fastread' go.mod && grep -qE '^go 1\.24(\.[0-9]+)?$' go.mod
CMD
expect_ok "toolchain line absent or go1.24.x" <<'CMD'
! grep -q '^toolchain' go.mod || grep -qE '^toolchain go1\.24(\.[0-9]+)?$' go.mod
CMD
expect_ok "GOTOOLCHAIN=local build" <<'CMD'
GOTOOLCHAIN=local go build ./...
CMD
expect_ok "TestDelayBase passes" <<'CMD'
passes TestDelayBase internal/timing
CMD
expect_ok "constants defined in internal/timing/constants.go" <<'CMD'
for c in SentenceMul ClauseMul LongWordStep LongWordCap LongWordFrom ParagraphMul MinWPM MaxWPM; do
  grep -qE "\\b$c\\b[^=]*=" internal/timing/constants.go || { echo "missing $c"; exit 1; }
done
CMD
expect_ok "exported Delay/DelayPara signatures" <<'CMD'
grep -qF 'func Delay(word string, wpm int) time.Duration' internal/timing/delay.go &&
grep -qF 'func DelayPara(word string, wpm int, paraEnd bool) time.Duration' internal/timing/delay.go
CMD
expect_fail "negative: go directive above 1.24 or newer toolchain" <<'CMD'
test -f go.mod || exit 0
grep -qE -e '^go 1\.(2[5-9]|[3-9][0-9])' -e '^toolchain go1\.(2[5-9]|[3-9][0-9])' go.mod
CMD
expect_fail "negative: internal/timing imports something outside the standard library" <<'CMD'
out="$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/timing)" || exit 0
printf '%s\n' "$out" | grep -v -x 'github.com/0x6c6d/fastread/internal/timing' | grep -q .
CMD
finish
