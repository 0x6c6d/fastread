#!/usr/bin/env bash
# Verification for T059 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestLayoutChrome TestLayoutFocusX TestLayoutBaseline TestLayoutTicks; do
expect_ok "$t passes" <<CMD
passes $t internal/gui
CMD
done
expect_ok "chrome and layout tests pass with -tags nogui and CGO_ENABLED=0" 600 <<'CMD'
go test -tags nogui -count=1 -run '^TestLayout' ./internal/gui &&
CGO_ENABLED=0 go test -count=1 -run '^TestLayout' ./internal/gui
CMD
expect_ok "declarations and the exact HelpText" <<'CMD'
d="$(go doc -all ./internal/gui)" || exit 1
for s in 'func LayoutChrome\(w, h int, pxPerSp float32, index, total int, m Measurer\) Chrome' \
  'func ProgressText\(index, total, wpm int, ok bool\) string' 'type Chrome struct' \
  'ChromeSp += 14' 'ChromeMarginPx += 8' 'BarHeightPx += 2' 'BarGapPx += 6'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
grep -qF 'const HelpText = "space pause  ↑↓ wpm  [ ] size  ←→ word  home restart  p progress  ? help  q quit"' internal/gui/chrome.go
CMD
expect_fail "negative: chrome.go has a build tag or imports gioui.org" <<'CMD'
test -f internal/gui/chrome.go || exit 0
head -1 internal/gui/chrome.go | grep -q '^//go:build' && exit 0
grep -n 'gioui.org' internal/gui/chrome.go internal/gui/chrome_test.go
CMD
finish
