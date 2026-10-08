#!/usr/bin/env bash
# Verification for T017 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestXHTMLText TestParseText TestLoadRawStdin; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "TestXHTMLText passes under -race" <<'CMD'
go test -race -count=1 -run '^TestXHTMLText$' ./internal/input
CMD
expect_ok "signature, tokenizer use, NextIsNotRawText" <<'CMD'
f=internal/input/xhtml.go
grep -qF 'func xhtmlParagraphs(b []byte) []string' "$f" &&
grep -qF 'html.NewTokenizer' "$f" &&
grep -qF 'NextIsNotRawText' "$f" &&
grep -qF '"golang.org/x/net/html"' "$f"
CMD
expect_ok "go.mod: x/net pinned at v0.50.0, go line still 1.24.x" <<'CMD'
grep -qE '^go 1\.24(\.[0-9]+)?$' go.mod &&
grep -qE '^[[:space:]]*golang\.org/x/net v0\.50\.0' go.mod &&
! grep -qE '^toolchain go1\.(2[5-9]|[3-9][0-9])' go.mod
CMD
expect_fail "negative: html.Parse tree builder used in internal/input" <<'CMD'
grep -nE 'html\.Parse(Fragment)?(WithOptions)?\(' $(ls internal/input/*.go | grep -v '_test\.go$')
CMD
expect_fail "negative: x/net/html imported outside xhtml.go" <<'CMD'
grep -rln --include='*.go' '"golang.org/x/net/html"' . | grep -v '^\./internal/input/xhtml\(_test\)\?\.go$'
CMD
expect_fail "negative: a direct dependency outside the pinned set" <<'CMD'
out="$(go list -m -f '{{if and (not .Main) (not .Indirect)}}{{.Path}}{{end}}' all)" || exit 0
printf '%s\n' "$out" | grep -v '^$' | grep -v -x -e 'gioui.org' -e 'golang.org/x/term' -e 'golang.org/x/sys' \
  -e 'golang.org/x/net' -e 'golang.org/x/image' -e 'golang.org/x/text' -e 'github.com/rivo/uniseg' \
  -e 'github.com/ledongthuc/pdf'
CMD
expect_fail "negative: internal/input depends on UI, Gio or network packages" <<'CMD'
out="$(go list -deps ./internal/input)" || exit 0
printf '%s\n' "$out" | grep -q -x -e 'net' -e 'net/http' -e 'github.com/0x6c6d/fastread/internal/tui' \
  -e 'github.com/0x6c6d/fastread/internal/gui' -e 'gioui.org.*'
CMD
finish
