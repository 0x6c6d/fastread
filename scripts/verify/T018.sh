#!/usr/bin/env bash
# Verification for T018 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestEPUBSpineOrder TestEPUBCorrupt TestEPUBLimits TestXHTMLText TestLoadFile TestDetectType; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "TestEPUB* pass under -race" <<'CMD'
go test -race -count=1 -run '^TestEPUB' ./internal/input
CMD
expect_ok "declarations and committed fixture" <<'CMD'
grep -qF 'func loadEPUB(b []byte, lim Limits) ([]string, error)' internal/input/epub.go &&
grep -qF 'func epubSpine(z *zipArchive, lim Limits) ([]string, error)' internal/input/epub.go &&
grep -qF 'func epubZip(t *testing.T, entries []epubEntry) []byte' internal/input/epub_fixture_test.go &&
grep -qF 'func sampleEPUBEntries() []epubEntry' internal/input/epub_fixture_test.go &&
f=internal/input/testdata/sample.epub && test -s "$f" &&
head -c 4 "$f" | od -An -tx1 | grep -q '50 4b 03 04' &&
grep -aqF 'OEBPS/content.opf' "$f" && grep -aqF 'META-INF/container.xml' "$f" &&
grep -aqF 'OEBPS/text/b.xhtml' "$f"
CMD
expect_ok "binary loads the sample EPUB (fails only for lack of a terminal)" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
setsid -w env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume internal/input/testdata/sample.epub </dev/null >"$T/out" 2>"$T/err"; rc=$?
cat "$T/err"
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 &&
! grep -qi -e 'corrupt' -e 'unsupported' -e 'empty' -e 'not found' "$T/err"
CMD
expect_ok "negative: truncated .epub exits 1 with one 'corrupt EPUB' line" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
f=internal/input/testdata/sample.epub
head -c $(( $(wc -c < "$f") / 2 )) "$f" > "$T/half.epub"
env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/half.epub" </dev/null >"$T/out" 2>"$T/err"; rc=$?
cat "$T/err"
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && grep -q 'corrupt EPUB' "$T/err" && test ! -e "$T/state"
CMD
expect_fail "negative: epub.go touches the filesystem or extracts entries" <<'CMD'
grep -nE -e '\bos\.' -e '\bfilepath\.' -e 'zip\.OpenReader' -e 'ioutil\.' internal/input/epub.go
CMD
finish
