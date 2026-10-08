#!/usr/bin/env bash
# Verification for T015 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestDetectType TestUnsupportedMismatch TestLoadFile TestParseText TestLoadRawStdin TestSelectBasic; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "declarations" <<'CMD'
for s in 'func DetectType\(name string, head \[\]byte\) \(FileType, error\)' 'TypeText FileType = iota \+ 1' \
  'func LoadFile\(path string\) \(Document, error\)' 'func loadFile\(path string, lim Limits\) \(Document, error\)' \
  'func loadMarkdown\(b \[\]byte\) \(\[\]string, error\)' 'func loadEPUB\(b \[\]byte, lim Limits\) \(\[\]string, error\)' \
  'func loadFB2\(b \[\]byte, lim Limits\) \(\[\]string, error\)' 'func loadPDF\(b \[\]byte, lim Limits\) \(\[\]string, error\)' \
  'func fixtureFile\(t \*testing.T, name string, data \[\]byte\) string' 'update-fixtures'; do
  grep -qE "$s" internal/input/*.go || { echo "missing: $s"; exit 1; }
done
CMD
expect_ok "file reads go through readFileCapped only" <<'CMD'
grep -q 'readFileCapped' internal/input/load.go &&
! grep -nE 'os\.ReadFile|ioutil\.ReadFile|io\.ReadAll' $(ls internal/input/*.go | grep -v -e '_test\.go$' -e 'safe\.go$')
CMD
expect_ok "negative (AC6): .pdf extension on a text file -> exit 1, one 'unsupported' line" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
printf 'hello world\n' > "$T/x.pdf"
printf 'PK\003\004 not really\n' > "$T/y.txt"
for f in "$T/x.pdf" "$T/y.txt"; do
  env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$f" </dev/null >"$T/out" 2>"$T/err"; rc=$?
  test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && grep -qi 'unsupported' "$T/err" || {
    echo "$f: rc=$rc"; cat "$T/err"; exit 1; }
done
CMD
expect_ok "negative: empty .txt file -> exit 1 'empty text'" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
: > "$T/e.txt"
env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/e.txt" </dev/null 2>"$T/err"; rc=$?
test "$rc" -eq 1 && grep -q 'empty text' "$T/err"
CMD
finish
