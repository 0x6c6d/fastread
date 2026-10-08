#!/usr/bin/env bash
# Verification for T057 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestLayoutFocusX TestLayoutBaseline TestLayoutTicks TestRunNoDisplay TestWindowNoDisplay; do
expect_ok "$t passes" <<CMD
passes $t internal/gui
CMD
done
expect_ok "layout tests pass with -tags nogui, CGO_ENABLED=0 and -race" 600 <<'CMD'
r='^(TestLayoutFocusX|TestLayoutBaseline|TestLayoutTicks)$'
for mode in nogui nocgo race; do
  case "$mode" in
    nogui) out="$(go test -tags nogui -count=1 -v -run "$r" ./internal/gui 2>&1)" ;;
    nocgo) out="$(CGO_ENABLED=0 go test -count=1 -v -run "$r" ./internal/gui 2>&1)" ;;
    race) out="$(go test -race -count=1 -v -run "$r" ./internal/gui 2>&1)" ;;
  esac || { printf '%s\n' "$out" | tail -n 30; exit 1; }
  for t in TestLayoutFocusX TestLayoutBaseline TestLayoutTicks; do
    printf '%s\n' "$out" | grep -q -- "^--- PASS: $t (" || { echo "$mode: no PASS for $t"; exit 1; }
  done
done
CMD
expect_ok "API declared: Layout, PxOf, Measurer, Metrics, LayoutInput/Result fields, LevelSp, constants" <<'CMD'
d="$(go doc -all ./internal/gui)" || exit 1
for s in 'func Layout\(in LayoutInput\) LayoutResult' 'func PxOf\(sp int, pxPerSp float32\) fixed\.Int26_6' \
  'type Measurer interface' 'Advances\(clusters \[\]string, px fixed\.Int26_6\) \[\]fixed\.Int26_6' \
  'Metrics\(px fixed\.Int26_6\) Metrics' 'type Metrics struct' 'LevelSp += \[6\]int\{0, 20, 32, 48, 64, 96\}' \
  'MinSp += 10' 'ShrinkStepSp += 2' 'TickGapPx += 4' 'TickWidthPx += 2' \
  'OriginX +fixed\.Int26_6' 'FocusX +fixed\.Int26_6' 'BaselineY +fixed\.Int26_6' 'TickTop.*image\.Rectangle' 'TickBottom.*image\.Rectangle' \
  'PxPerSp +float32' 'Parts +\[\]string'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
grep -q 'func newFontMeasurer(t testing.TB) Measurer' internal/gui/measure_test.go &&
grep -q 'gofont/goregular' internal/gui/measure_test.go && grep -q 'type fixedMeasurer' internal/gui/measure_test.go
CMD
expect_ok "all internal/gui tests still pass (cgo, nogui)" 600 <<'CMD'
go test -count=1 ./internal/gui && go test -tags nogui -count=1 ./internal/gui
CMD
expect_fail "negative: layout.go has a build tag or imports gioui.org" <<'CMD'
test -f internal/gui/layout.go || exit 0
head -1 internal/gui/layout.go | grep -q '^//go:build' && exit 0
grep -n 'gioui.org' internal/gui/layout.go internal/gui/measure_test.go internal/gui/layout_test.go
CMD
expect_fail "negative: nogui or no-cgo build of internal/gui depends on gioui.org" <<'CMD'
a="$(go list -tags nogui -deps ./internal/gui)" || exit 0
b="$(CGO_ENABLED=0 go list -deps ./internal/gui)" || exit 0
printf '%s\n%s\n' "$a" "$b" | grep -q '^gioui.org'
CMD
finish
