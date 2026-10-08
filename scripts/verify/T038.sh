#!/usr/bin/env bash
# Verification for T038 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestSizeFallback TestGlyphFallback TestRenderFocusLevel1 TestRenderClip TestEncodeColors TestDetectColorMode; do
expect_ok "$t passes" <<CMD
passes $t internal/tui
CMD
done
expect_ok "EffectiveSize signature and Frame.Size field" <<'CMD'
d="$(go doc -all ./internal/tui)" || exit 1
printf '%s\n' "$d" | grep -qE 'func EffectiveSize\(word string, size, w, h int\) int' &&
go doc ./internal/tui Frame | grep -qE '^[[:space:]]+Size +int'
CMD
expect_ok "TestSizeFallback covers the AC14 sizes" <<'CMD'
f=internal/tui/glyph_render_test.go; test -f "$f" || exit 1
for s in '19' '29' '28' '23' '22'; do grep -q -- "$s" "$f" || { echo "missing width $s"; exit 1; }; done
CMD
expect_fail "negative: render/layout files import os or internal/state" <<'CMD'
grep -n -e '"os"' -e 'fastread/internal/state"' internal/tui/render.go internal/tui/layout.go
CMD
finish
