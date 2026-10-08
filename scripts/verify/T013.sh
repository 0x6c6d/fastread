#!/usr/bin/env bash
# Verification for T013 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestSafeReadCapped TestSafeZip TestSafeXML TestSafeRecover TestSafeLoadStdinCap; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "TestSafe* pass under -race" <<'CMD'
go test -race -count=1 -run '^TestSafe' ./internal/input
CMD
expect_ok "safe.go declarations" <<'CMD'
f=internal/input/safe.go; test -f "$f" || exit 1
for s in 'MaxInputBytes *= *256 << 20' 'MaxZipEntries *= *10000' 'MaxXMLDepth *= *256' \
  'func DefaultLimits\(\) Limits' 'func readCapped\(r io.Reader, max int64\) \(\[\]byte, error\)' \
  'func readFileCapped\(path string, max int64\) \(\[\]byte, error\)' \
  'func openZip\(b \[\]byte, lim Limits\) \(\*zipArchive, error\)' \
  'func \(z \*zipArchive\) read\(name string\) \(\[\]byte, error\)' \
  'func newXMLDecoder\(r io.Reader\) \*xml.Decoder' \
  'func walkXML\(d \*xml.Decoder, maxDepth int, fn func\(tok xml.Token, depth int\) error\) error' \
  'func recoverAs\(sentinel error, fn func\(\) error\) \(err error\)' \
  'func load\(src Source, stdin io.Reader, lim Limits\) \(Document, error\)'; do
  grep -qE "$s" internal/input/*.go || { echo "missing: $s"; exit 1; }
done
CMD
expect_ok "XML decoder configured as required" <<'CMD'
grep -q 'Strict *= *false' internal/input/safe.go &&
grep -q 'xml.HTMLEntity' internal/input/safe.go &&
grep -q 'xml.HTMLAutoClose' internal/input/safe.go &&
grep -q 'CharsetReader' internal/input/safe.go
CMD
expect_fail "negative: input code writes files or extracts zips" <<'CMD'
grep -nE -e 'zip\.OpenReader' -e 'os\.(Create|WriteFile|Mkdir|MkdirAll|OpenFile)\(' \
  $(ls internal/input/*.go | grep -v '_test\.go$')
CMD
expect_fail "negative: xml.NewDecoder used outside safe.go" <<'CMD'
grep -n 'xml\.NewDecoder' $(ls internal/input/*.go | grep -v -e '_test\.go$' -e 'safe\.go$')
CMD
expect_fail "negative: internal/input depends on UI, network or non-stdlib code" <<'CMD'
out="$(go list -deps ./internal/input)" || exit 0
printf '%s\n' "$out" | grep -q -x -e 'net' -e 'net/http' -e 'github.com/0x6c6d/fastread/internal/tui' \
  -e 'github.com/0x6c6d/fastread/internal/gui' -e 'gioui.org.*'
CMD
finish
