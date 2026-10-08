#!/usr/bin/env bash
# Verification for T039 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestTicks TestTooSmall TestSizeFallback TestGlyphFallback TestRenderFocusLevel1 TestRenderClip; do
expect_ok "$t passes" <<CMD
passes $t internal/tui
CMD
done
expect_ok "TooSmallText constant and tick rune" <<'CMD'
go doc ./internal/tui TooSmallText | grep -qF '"terminal too small"' &&
grep -qF '│' internal/tui/render.go
CMD
finish
