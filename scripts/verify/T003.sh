#!/usr/bin/env bash
# Verification for T003 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestPositionBasic passes" <<'CMD'
passes TestPositionBasic internal/orp
CMD
expect_ok "TestIndexBasic passes" <<'CMD'
passes TestIndexBasic internal/orp
CMD
expect_ok "TestNoPanic passes" <<'CMD'
passes TestNoPanic internal/orp
CMD
expect_ok "uniseg pinned at v0.4.7, go 1.24.x directive, GOTOOLCHAIN=local build" <<'CMD'
grep -qE '^[[:space:]]*(require[[:space:]]+)?github.com/rivo/uniseg v0\.4\.7$' go.mod &&
grep -q '^github.com/rivo/uniseg v0.4.7 ' go.sum &&
grep -qE '^go 1\.24(\.[0-9]+)?$' go.mod &&
GOTOOLCHAIN=local go build ./... && go mod verify
CMD
expect_ok "exported signatures" <<'CMD'
grep -qF 'func Position(word string) int' internal/orp/orp.go &&
grep -qF 'func Index(word string) int' internal/orp/orp.go &&
grep -qF 'func Clusters(word string) []string' internal/orp/orp.go
CMD
expect_fail "negative: go directive above 1.24 or newer toolchain" <<'CMD'
test -f go.mod || exit 0
grep -qE -e '^go 1\.(2[5-9]|[3-9][0-9])' -e '^toolchain go1\.(2[5-9]|[3-9][0-9])' go.mod
CMD
expect_fail "negative: internal/orp imports something besides stdlib and uniseg" <<'CMD'
out="$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/orp)" || exit 0
printf '%s\n' "$out" | grep -v -x -e 'github.com/0x6c6d/fastread/internal/orp' -e 'github.com/rivo/uniseg' | grep -q .
CMD
finish
