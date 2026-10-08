#!/usr/bin/env bash
# Verification for T058 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestLayoutShrinkSplit TestLayoutFocusXAllSizes TestLayoutFocusX TestLayoutBaseline TestLayoutTicks; do
expect_ok "$t passes" <<CMD
passes $t internal/gui
CMD
done
expect_ok "TestLayoutFocusXAllSizes logs >= 30 words, widths 200,600,601,1920 and >= 1200 cases" <<'CMD'
out="$(go test -count=1 -v -run '^TestLayoutFocusXAllSizes$' ./internal/gui 2>&1)" || { printf '%s\n' "$out" | tail -n 20; exit 1; }
line="$(printf '%s\n' "$out" | grep -oE 'words=[0-9]+ levels=5 widths=200,600,601,1920 cases=[0-9]+' | head -1)"
test -n "$line" || { echo "no words=/cases= log line"; exit 1; }
w="$(printf '%s\n' "$line" | sed 's/^words=\([0-9]*\).*/\1/')"; c="${line##*cases=}"
echo "$line"; test "$w" -ge 30 && test "$c" -ge 1200
CMD
expect_ok "layout tests pass with -tags nogui, CGO_ENABLED=0 and -race" 900 <<'CMD'
r='^TestLayout'
go test -tags nogui -count=1 -run "$r" ./internal/gui &&
CGO_ENABLED=0 go test -count=1 -run "$r" ./internal/gui &&
go test -race -count=1 -run "$r" ./internal/gui
CMD
expect_ok "race constant files" <<'CMD'
head -1 internal/gui/race_on_test.go | grep -qx '//go:build race' && grep -q 'raceEnabled = true' internal/gui/race_on_test.go &&
head -1 internal/gui/race_off_test.go | grep -qx '//go:build !race' && grep -q 'raceEnabled = false' internal/gui/race_off_test.go
CMD
expect_ok "all internal/gui tests still pass" 600 <<'CMD'
go test -count=1 ./internal/gui && go test -tags nogui -count=1 ./internal/gui
CMD
expect_fail "negative: layout.go has a build tag or imports gioui.org; nogui build depends on Gio" <<'CMD'
test -f internal/gui/layout.go || exit 0
head -1 internal/gui/layout.go | grep -q '^//go:build' && exit 0
grep -n 'gioui.org' internal/gui/layout.go internal/gui/shrink_test.go && exit 0
out="$(go list -tags nogui -deps ./internal/gui)" || exit 0
printf '%s\n' "$out" | grep -q '^gioui.org'
CMD
finish
