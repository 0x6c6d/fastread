#!/usr/bin/env bash
# Verification for T027 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestTokenize TestSanitize TestTokenizeBasic; do
expect_ok "$t passes" <<CMD
passes $t internal/tokenize
CMD
done
expect_ok "tokenize tests pass under -race" <<'CMD'
go test -race -count=1 ./internal/tokenize
CMD
expect_ok "Sanitize and Tokenize signatures" <<'CMD'
grep -qF 'func Sanitize(s string) string' internal/tokenize/*.go &&
grep -qF 'func Tokenize(paragraphs []string) []Token' internal/tokenize/tokenize.go &&
grep -qF 'func FuzzTokenize(f *testing.F)' internal/tokenize/tokenize_test.go
CMD
expect_ok "negative: FuzzTokenize finds no leaking input in 15 s" 180 <<'CMD'
go test -count=1 -run '^$' -fuzz '^FuzzTokenize$' -fuzztime 15s ./internal/tokenize
CMD
expect_ok "internal/tokenize uses only the standard library" <<'CMD'
out="$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/tokenize)" || exit 1
test -z "$(printf '%s\n' "$out" | grep -v -x 'github.com/0x6c6d/fastread/internal/tokenize' | grep .)"
CMD
expect_fail "negative: regexp used in internal/tokenize" <<'CMD'
grep -n '"regexp"' $(ls internal/tokenize/*.go | grep -v '_test\.go$')
CMD
expect_fail "negative: internal/tokenize imports a UI package or Gio" <<'CMD'
out="$(go list -deps ./internal/tokenize)" || exit 0
printf '%s\n' "$out" | grep -q -e fastread/internal/tui -e fastread/internal/gui -e gioui.org
CMD
expect_ok "no fuzz crasher left in the tree" <<'CMD'
test ! -d internal/tokenize/testdata/fuzz/FuzzTokenize || test -z "$(git status --porcelain internal/tokenize/testdata/fuzz)"
CMD
finish
