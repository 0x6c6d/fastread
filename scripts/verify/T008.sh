#!/usr/bin/env bash
# Verification for T008 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestRunNoDisplay passes (cgo build)" <<'CMD'
passes TestRunNoDisplay internal/gui
CMD
expect_ok "TestWindowNoDisplay passes (cgo build)" <<'CMD'
passes TestWindowNoDisplay internal/gui
CMD
expect_ok "TestRunNoDisplay and TestStubUnavailable pass with -tags nogui" <<'CMD'
out="$(go test -tags nogui -count=1 -v -run '^(TestRunNoDisplay|TestStubUnavailable)$' ./internal/gui 2>&1)" || { printf '%s\n' "$out"; exit 1; }
printf '%s\n' "$out" | grep -q -- '^--- PASS: TestRunNoDisplay (' &&
printf '%s\n' "$out" | grep -q -- '^--- PASS: TestStubUnavailable ('
CMD
expect_ok "TestRunNoDisplay and TestStubUnavailable pass with CGO_ENABLED=0" <<'CMD'
out="$(CGO_ENABLED=0 go test -count=1 -v -run '^(TestRunNoDisplay|TestStubUnavailable)$' ./internal/gui 2>&1)" || { printf '%s\n' "$out"; exit 1; }
printf '%s\n' "$out" | grep -q -- '^--- PASS: TestRunNoDisplay (' &&
printf '%s\n' "$out" | grep -q -- '^--- PASS: TestStubUnavailable ('
CMD
expect_ok "build tags on stub.go and window.go" <<'CMD'
head -1 internal/gui/stub.go | grep -qx '//go:build nogui || !cgo' &&
head -1 internal/gui/window.go | grep -qx '//go:build !nogui && cgo'
CMD
expect_ok "pins: gioui.org v0.10.3, x/image v0.36.0, x/sys v0.41.0, x/term v0.40.0, go 1.24.x" <<'CMD'
grep -qE '^[[:space:]]*(require[[:space:]]+)?gioui.org v0\.10\.3$' go.mod &&
grep -qE '^[[:space:]]*(require[[:space:]]+)?golang.org/x/image v0\.36\.0$' go.mod &&
grep -qE '^[[:space:]]*(require[[:space:]]+)?golang.org/x/sys v0\.41\.0( // indirect)?$' go.mod &&
grep -qE '^[[:space:]]*(require[[:space:]]+)?golang.org/x/term v0\.40\.0$' go.mod &&
grep -qE '^go 1\.24(\.[0-9]+)?$' go.mod &&
! grep -qE '^toolchain go1\.(2[5-9]|[3-9][0-9])' go.mod &&
GOTOOLCHAIN=local go build ./... && go mod verify
CMD
expect_ok "Go Regular font from x/image, no system fonts" <<'CMD'
grep -q 'golang.org/x/image/font/gofont/goregular' internal/gui/window.go &&
grep -q 'NoSystemFonts' internal/gui/window.go
CMD
expect_fail "negative: nogui build of internal/gui depends on gioui.org" <<'CMD'
out="$(go list -tags nogui -deps ./internal/gui)" || exit 0
printf '%s\n' "$out" | grep -q '^gioui.org'
CMD
expect_fail "negative: CGO_ENABLED=0 build of internal/gui depends on gioui.org" <<'CMD'
out="$(CGO_ENABLED=0 go list -deps ./internal/gui)" || exit 0
printf '%s\n' "$out" | grep -q '^gioui.org'
CMD
expect_fail "negative: a .go file outside internal/gui mentions gioui.org" <<'CMD'
test -d internal/gui || exit 0
grep -rl gioui.org --include='*.go' . | grep -v '^./internal/gui/'
CMD
expect_fail "negative: gui.go/stub.go import Gio, or os.Exit in internal/gui" <<'CMD'
test -f internal/gui/gui.go -a -f internal/gui/stub.go || exit 0
grep -n gioui.org internal/gui/gui.go internal/gui/stub.go; a=$?
grep -n 'os\.Exit' internal/gui/*.go; b=$?
test $a -ne 0 -a $b -ne 0 && exit 1
exit 0
CMD
finish
