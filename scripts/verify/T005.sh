#!/usr/bin/env bash
# Verification for T005 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestParseText TestSelectBasic TestLoadRawStdin TestErrorMessages; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "sentinels and API declared" <<'CMD'
d="$(go doc -all ./internal/input)" || exit 1
for s in ErrNotFound ErrCorruptEPUB ErrEmpty ErrUnsupported ErrNoText ErrCorruptPDF ErrTooLarge ErrNoInput \
  'func Select\(arg string, hasArg, stdinIsTTY bool\) \(Source, error\)' \
  'func Load\(src Source, stdin io.Reader\) \(Document, error\)' \
  'func ParseText\(b \[\]byte\) \[\]string' 'SHA256 +\[32\]byte' 'KindStdin' 'KindFile' 'KindRaw'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
CMD
expect_fail "negative: internal/input imports a UI package or Gio" <<'CMD'
out="$(go list -deps ./internal/input)" || exit 0
printf '%s\n' "$out" | grep -q -e fastread/internal/tui -e fastread/internal/gui -e gioui.org
CMD
expect_fail "negative: internal/input has a non-stdlib dependency in the skeleton" <<'CMD'
out="$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/input)" || exit 0
printf '%s\n' "$out" | grep -v -x 'github.com/0x6c6d/fastread/internal/input' | grep -q .
CMD
finish
