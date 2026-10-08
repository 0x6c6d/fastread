#!/usr/bin/env bash
# Verification for T028 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestPosition TestIndex TestIndexInRange TestPositionBasic TestIndexBasic TestNoPanic; do
expect_ok "$t passes" <<CMD
passes $t internal/orp
CMD
done
expect_ok "AC7 words present in the table test" <<'CMD'
f=internal/orp/orp_table_test.go; test -f "$f" || exit 1
for w in '"a"' '"ab"' '"hello"' '"wonderful"' '"reading,"' 'extraordinary' '"internationally"' '"--"' \
  'naïve' '1,000,000'; do
  grep -qF -- "$w" "$f" || { echo "missing row: $w"; exit 1; }
done
CMD
expect_ok "orp exported API unchanged" <<'CMD'
grep -qF 'func Position(word string) int' internal/orp/orp.go &&
grep -qF 'func Index(word string) int' internal/orp/orp.go &&
grep -qF 'func Clusters(word string) []string' internal/orp/orp.go
CMD
expect_fail "negative: internal/orp imports something besides stdlib and uniseg" <<'CMD'
out="$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/orp)" || exit 0
printf '%s\n' "$out" | grep -v -x -e 'github.com/0x6c6d/fastread/internal/orp' -e 'github.com/rivo/uniseg' | grep -q .
CMD
finish
