#!/usr/bin/env bash
# Verification for T029 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestDelay TestDelayPara TestDelayBase; do
expect_ok "$t passes" <<CMD
passes $t internal/timing
CMD
done
expect_ok "signatures unchanged, constants unchanged, TODO marker removed" <<'CMD'
grep -qF 'func Delay(word string, wpm int) time.Duration' internal/timing/delay.go &&
grep -qF 'func DelayPara(word string, wpm int, paraEnd bool) time.Duration' internal/timing/delay.go &&
grep -qE 'SentenceMul *= *2\.0' internal/timing/constants.go && grep -qE 'ParagraphMul *= *2\.5' internal/timing/constants.go &&
! grep -q 'TODO(phase3)' internal/timing/delay.go
CMD
expect_ok "delay.go uses the named constants" <<'CMD'
for c in SentenceMul ClauseMul LongWordStep LongWordCap LongWordFrom ParagraphMul; do
  grep -q "\\b$c\\b" internal/timing/delay.go || { echo "delay.go does not use $c"; exit 1; }
done
CMD
expect_fail "negative: literal multipliers in delay.go" <<'CMD'
grep -nE -e '\b2\.5\b' -e '\b1\.5\b' -e '\b2\.0\b' -e '\b0\.05\b' internal/timing/delay.go
CMD
expect_fail "negative: internal/timing imports something outside the standard library" <<'CMD'
out="$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/timing)" || exit 0
printf '%s\n' "$out" | grep -v -x 'github.com/0x6c6d/fastread/internal/timing' | grep -q .
CMD
finish
