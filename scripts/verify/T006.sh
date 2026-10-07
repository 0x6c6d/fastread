#!/usr/bin/env bash
# Verification for T006 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestRenderFocusLevel1 passes" <<'CMD'
passes TestRenderFocusLevel1 internal/tui
CMD
expect_ok "TestRenderClip passes" <<'CMD'
passes TestRenderClip internal/tui
CMD
expect_ok "TestEncodeColors passes" <<'CMD'
passes TestEncodeColors internal/tui
CMD
expect_ok "TestDetectColorMode passes" <<'CMD'
passes TestDetectColorMode internal/tui
CMD
expect_ok "exported API present" <<'CMD'
d="$(go doc -all ./internal/tui)" || exit 1
for s in 'func Render\(m Model, w, h int\) Frame' 'func Encode\(f Frame, mode ColorMode\) \[\]byte' \
  'func DetectColorMode\(getenv func\(string\) string\) ColorMode' 'func FocusColumn\(w int\) int' \
  'ColorTrue' 'StyleFocus' 'StyleTick' 'Cont +bool'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
CMD
expect_ok "internal/tui imports no Gio and no internal/gui" <<'CMD'
out="$(go list -deps ./internal/tui)" || exit 1
! printf '%s\n' "$out" | grep -q -e fastread/internal/gui -e gioui.org
CMD
finish
