#!/usr/bin/env bash
# Verification for T060 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestGUIKeyMap TestRunNoDisplay TestWindowNoDisplay; do
expect_ok "$t passes" <<CMD
passes $t internal/gui
CMD
done
expect_ok "TestGUIKeyMap passes under -race; nogui tests still pass" 600 <<'CMD'
go test -race -count=1 -run '^TestGUIKeyMap$' ./internal/gui && go test -tags nogui -count=1 ./internal/gui
CMD
expect_ok "build tags and signatures in keys.go" <<'CMD'
head -1 internal/gui/keys.go | grep -qx '//go:build !nogui && cgo' &&
head -1 internal/gui/keys_test.go | grep -qx '//go:build !nogui && cgo' &&
grep -qE 'func keyFilters\(\) \[\]event\.Filter' internal/gui/keys.go &&
grep -qE 'func keyAction\(e key\.Event\) \(a state\.Action, ok bool\)' internal/gui/keys.go
CMD
expect_fail "negative: nogui or no-cgo build of internal/gui depends on gioui.org" <<'CMD'
a="$(go list -tags nogui -deps ./internal/gui)" || exit 0
b="$(CGO_ENABLED=0 go list -deps ./internal/gui)" || exit 0
printf '%s\n%s\n' "$a" "$b" | grep -q '^gioui.org'
CMD
expect_fail "negative: a .go file outside internal/gui mentions gioui.org, or os.Exit in internal/gui" <<'CMD'
grep -rl gioui.org --include='*.go' . | grep -v '^./internal/gui/' && exit 0
grep -n 'os\.Exit' internal/gui/*.go
CMD
finish
