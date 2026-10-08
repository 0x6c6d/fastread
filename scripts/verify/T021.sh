#!/usr/bin/env bash
# Verification for T021 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestPDFText TestPDFNoText TestPDFBasicCorrupt TestDetectType TestLoadFile; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "TestPDF* pass under -race" <<'CMD'
go test -race -count=1 -run '^TestPDF' ./internal/input
CMD
expect_ok "pin, go line, single import site, one recoverAs wrapper" <<'CMD'
grep -qE '^go 1\.24(\.[0-9]+)?$' go.mod &&
grep -qE '^[[:space:]]*github\.com/ledongthuc/pdf v0\.0\.0-20260907135840-6c8c28e0e8a0' go.mod &&
test "$(grep -rl --include='*.go' '"github.com/ledongthuc/pdf"' . )" = './internal/input/pdf.go' &&
test "$(grep -c 'recoverAs(ErrCorruptPDF' internal/input/pdf.go)" -eq 1 &&
grep -qE 'MaxPDFPages *= *10000' internal/input/pdf.go &&
grep -qF 'func loadPDF(b []byte, lim Limits) ([]string, error)' internal/input/pdf.go &&
grep -qF 'func buildPDF(pages []string) []byte' internal/input/pdf_fixture_test.go
CMD
expect_ok "committed fixtures" <<'CMD'
for f in sample notext; do
  p="internal/input/testdata/$f.pdf"; test -s "$p" || exit 1
  head -c 5 "$p" | grep -q '%PDF-' && grep -aq 'startxref' "$p" && grep -aq '%%EOF' "$p" || exit 1
done
grep -aqF '(Hello PDF world.) Tj' internal/input/testdata/sample.pdf
CMD
expect_fail "negative: pdf.go uses trusting/unbounded library entry points" <<'CMD'
grep -nE -e 'NumPage\(' -e '\.GetPlainText\(\)' -e 'io\.ReadAll' -e '[A-Za-z]\.Page\(' -e 'GetStyledTexts' internal/input/pdf.go
CMD
expect_ok "negative: binary on notext.pdf exits 1 with one 'no extractable text' line" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume internal/input/testdata/notext.pdf </dev/null >"$T/out" 2>"$T/err"; rc=$?
cat "$T/err"
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && grep -q 'no extractable text' "$T/err" && test ! -s "$T/out"
CMD
expect_ok "binary loads sample.pdf (fails only for lack of a terminal)" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
setsid -w env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume internal/input/testdata/sample.pdf </dev/null 2>"$T/err"; rc=$?
cat "$T/err"
test "$rc" -eq 1 && ! grep -qi -e 'corrupt' -e 'no extractable' -e 'unsupported' -e 'empty' "$T/err"
CMD
finish
