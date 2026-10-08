#!/usr/bin/env bash
# Verification for T040 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestWideAndCombining TestRenderFocusLevel1 TestRenderClip TestSizeFallback TestGlyphFallback TestTicks TestTooSmall; do
expect_ok "$t passes" <<CMD
passes $t internal/tui
CMD
done
expect_ok "zero-width clusters drawn with U+25CC" <<'CMD'
grep -q -e '◌' -e '\\u25cc' -e '\\u25CC' internal/tui/render.go
CMD
finish
