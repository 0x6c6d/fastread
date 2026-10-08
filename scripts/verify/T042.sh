#!/usr/bin/env bash
# Verification for T042 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestProgressHelpFocus TestNoProgressFrame TestTicks TestTooSmall TestSizeFallback TestGlyphFallback TestWideAndCombining TestRenderFocusLevel1; do
expect_ok "$t passes" <<CMD
passes $t internal/tui
CMD
done
expect_ok "HelpText constant is exact" <<'CMD'
go doc ./internal/tui HelpText | grep -qF '"space pause  ↑↓ wpm  [ ] size  ←→ word  home restart  p progress  ? help  q quit"'
CMD
expect_ok "literal progress rows used in the tests" <<'CMD'
f=internal/tui/rows_test.go; test -f "$f" || exit 1
grep -qF 'word 5/120  287 wpm' "$f" && grep -qF '— wpm' "$f" && grep -qF 'word 1/' "$f"
CMD
finish
