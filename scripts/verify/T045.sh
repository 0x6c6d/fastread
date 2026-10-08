#!/usr/bin/env bash
# Verification for T045 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Also the Phase 4 exit criterion of Plan.md.
. "$(dirname "$0")/_lib.sh"
expect_ok "Phase 4 exit criterion (check + named tests + benchmark)" 1500 <<'CMD'
check && for t in TestGolden TestGlyphFallback TestFocusColumn TestColorModes TestNoColor TestSizeFallback TestTooSmall TestWideAndCombining TestTicks TestSplitLongWord TestProgressHelpFocus TestNoProgressFrame TestEncodeStripsControls TestRenderFast; do passes $t internal/tui || exit 1; done && go test -run '^$' -bench '^BenchmarkRenderFrame$' -benchtime 100x ./internal/tui/ | grep -q '^BenchmarkRenderFrame'
CMD
expect_ok "TestRenderInvariants passes (also -race and -tags nogui)" 600 <<'CMD'
passes TestRenderInvariants internal/tui &&
go test -race -count=1 -run '^TestRenderInvariants$' ./internal/tui &&
go test -tags nogui -count=1 -run '^TestRenderInvariants$' ./internal/tui
CMD
expect_ok "glyph tables still pass" <<'CMD'
passes TestGlyphTables internal/tui/glyph
CMD
expect_ok "TestRenderInvariants runs at least 3000 cases" <<'CMD'
out="$(go test -count=1 -v -run '^TestRenderInvariants$' ./internal/tui 2>&1)" || { printf '%s\n' "$out" | tail -20; exit 1; }
n="$(printf '%s\n' "$out" | grep -oE 'cases: [0-9]+' | grep -oE '[0-9]+' | tail -1)"
test -n "$n" && test "$n" -ge 3000
CMD
expect_ok "protected files unchanged" <<'CMD'
scripts/check-protected.sh
CMD
expect_fail "negative: internal/tui imports Gio, internal/gui or network packages" <<'CMD'
out="$(go list -deps ./internal/tui ./internal/tui/glyph)" || exit 0
printf '%s\n' "$out" | grep -q -x -e 'net' -e 'net/http' -e 'github.com/0x6c6d/fastread/internal/gui' -e 'gioui.org.*'
CMD
expect_fail "negative: logic packages import internal/tui" <<'CMD'
out="$(go list -deps ./internal/input ./internal/tokenize ./internal/orp ./internal/timing ./internal/state)" || exit 0
printf '%s\n' "$out" | grep -q -e 'fastread/internal/tui'
CMD
finish
