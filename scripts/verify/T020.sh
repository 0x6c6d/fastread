#!/usr/bin/env bash
# Verification for T020 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestFB2BodyOnly TestDetectType TestLoadFile TestUnsupportedMismatch; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "TestFB2BodyOnly passes under -race" <<'CMD'
go test -race -count=1 -run '^TestFB2BodyOnly$' ./internal/input
CMD
expect_ok "signature, helper use and committed fixture" <<'CMD'
grep -qF 'func loadFB2(b []byte, lim Limits) ([]string, error)' internal/input/fb2.go &&
grep -qF 'walkXML(' internal/input/fb2.go && grep -qF 'newXMLDecoder(' internal/input/fb2.go &&
f=internal/input/testdata/sample.fb2 && test -s "$f" &&
grep -qF '<book-title>Hidden Title</book-title>' "$f" && grep -qF '<binary id="c.jpg"' "$f" &&
grep -qF '<p>Second <emphasis>body</emphasis> paragraph.</p>' "$f" &&
grep -qF '<body name="notes">' "$f"
CMD
expect_fail "negative: xml.NewDecoder used in fb2.go" <<'CMD'
grep -n 'xml\.NewDecoder' internal/input/fb2.go
CMD
expect_ok "negative: description-only FB2 -> exit 1 'empty text'; 300-deep FB2 -> exit 1 'too large'" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
printf '<?xml version="1.0" encoding="UTF-8"?>\n<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description><title-info><book-title>Only Here</book-title></title-info></description><body></body></FictionBook>\n' > "$T/d.fb2"
env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/d.fb2" </dev/null 2>"$T/e1"; r1=$?
cat "$T/e1"
{ printf '<FictionBook><body>'; for i in $(seq 300); do printf '<section>'; done; printf '<p>deep</p>'; } > "$T/deep.fb2"
env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/deep.fb2" </dev/null 2>"$T/e2"; r2=$?
cat "$T/e2"
test "$r1" -eq 1 && grep -q 'empty text' "$T/e1" &&
test "$r2" -eq 1 && test "$(wc -l < "$T/e2")" -eq 1 && grep -q 'too large' "$T/e2"
CMD
finish
