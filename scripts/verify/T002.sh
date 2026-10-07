#!/usr/bin/env bash
# Verification for T002 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestTokenizeBasic passes" <<'CMD'
passes TestTokenizeBasic internal/tokenize
CMD
expect_ok "exported Token and Tokenize" <<'CMD'
grep -qF 'func Tokenize(paragraphs []string) []Token' internal/tokenize/tokenize.go &&
go doc ./internal/tokenize Token | grep -q 'Text *string' &&
go doc ./internal/tokenize Token | grep -q 'ParaEnd *bool'
CMD
expect_ok "internal/tokenize uses only the standard library" <<'CMD'
out="$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/tokenize)" || exit 1
test -z "$(printf '%s\n' "$out" | grep -v -x 'github.com/0x6c6d/fastread/internal/tokenize' | grep .)"
CMD
finish
