#!/usr/bin/env bash
# Verification for T064 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "e2e files carry the e2e build tag, are test files, vet clean" <<'CMD'
test -f e2e/guierr_test.go || exit 1
for f in e2e/*.go; do
  case "$f" in *_test.go) ;; *) echo "$f is not a _test.go file"; exit 1;; esac
  head -1 "$f" | grep -qx '//go:build e2e' || { echo "$f lacks //go:build e2e"; exit 1; }
done
go vet -tags e2e ./e2e/
CMD
expect_ok "TestE2EGUINoDisplay, TestE2EGUIBadDisplay, TestE2ENoguiBinary (+ subtests), TestE2EBasic pass; nothing left; real state dir untouched" 600 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
out="$(go test -tags e2e -count=1 -timeout 500s -v -run '^(TestE2EGUINoDisplay|TestE2EGUIBadDisplay|TestE2ENoguiBinary|TestE2EBasic)$' ./e2e/ 2>&1)"; rc=$?
printf '%s\n' "$out" | tail -n 50
test "$rc" -eq 0 || exit 1
for n in TestE2EGUINoDisplay TestE2EGUINoDisplay/raw TestE2EGUINoDisplay/file TestE2EGUIBadDisplay TestE2EGUIBadDisplay/x11 TestE2EGUIBadDisplay/wayland TestE2ENoguiBinary TestE2EBasic; do
  printf '%s\n' "$out" | grep -qE -- "^ *--- PASS: $n \(" || { echo "no PASS line for $n"; exit 1; }
done
sleep 0.5
! pgrep -u "$(id -u)" -x fastread >/dev/null || { echo "fastread process left running"; exit 1; }
if ls "/tmp/tmux-$(id -u)" 2>/dev/null | grep -q '^fastread-e2e-'; then
  for s in $(ls "/tmp/tmux-$(id -u)" | grep '^fastread-e2e-'); do
    tmux -L "$s" ls >/dev/null 2>&1 && { echo "tmux server $s still running"; exit 1; }
  done
fi
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || { echo "real state dir created"; exit 1; }
CMD
expect_ok "Plan D11 (nogui binary) verbatim" 300 <<'CMD'
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
CGO_ENABLED=0 go build -tags nogui -o $T/fr ./cmd/fastread && ! go version -m $T/fr | grep -q gioui.org && ! go tool nm $T/fr | grep -q gioui.org && { env -u DISPLAY -u WAYLAND_DISPLAY $T/fr --ui gui x 2>$T/err; test $? -eq 1; } && test "$(wc -l < $T/err)" -eq 1
CMD
expect_ok "Plan D12 (no-display GUI) verbatim" 300 <<'CMD'
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
go build -o $T/fg ./cmd/fastread && { env -u DISPLAY -u WAYLAND_DISPLAY $T/fg --ui gui x 2>$T/err2; test $? -eq 1; } && test "$(wc -l < $T/err2)" -eq 1
CMD
expect_fail "negative: guierr_test.go uses a fixed display number or the user's DISPLAY" <<'CMD'
grep -nE -e 'DISPLAY=:[0-9]' -e 'Getenv\("DISPLAY"\)' -e 'LookupEnv\("DISPLAY"\)' e2e/guierr_test.go
CMD
finish
