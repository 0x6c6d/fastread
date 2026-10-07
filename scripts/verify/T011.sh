#!/usr/bin/env bash
# Verification for T011 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Also the Phase 1 exit criterion from Plan.md.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "e2e files carry the e2e build tag and are test files" <<'CMD'
ls e2e/*.go >/dev/null 2>&1 || exit 1
for f in e2e/*.go; do
  case "$f" in *_test.go) ;; *) echo "$f is not a _test.go file"; exit 1;; esac
  head -1 "$f" | grep -qx '//go:build e2e' || { echo "$f lacks //go:build e2e"; exit 1; }
done
go vet -tags e2e ./e2e/
CMD
expect_ok "Phase 1 exit criterion incl. TestE2EBasic; no leftovers; real state dir untouched" 400 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
check && GOTOOLCHAIN=local go build ./... || exit 1
out="$(go test -tags e2e -count=1 -timeout 120s -v -run '^TestE2EBasic$' ./e2e/ 2>&1)"; rc=$?
printf '%s\n' "$out" | tail -n 40
test "$rc" -eq 0 && printf '%s\n' "$out" | grep -q -- '^--- PASS: TestE2EBasic (' || exit 1
sleep 0.5
if ls "/tmp/tmux-$(id -u)" 2>/dev/null | grep -q '^fastread-e2e-'; then
  for s in $(ls "/tmp/tmux-$(id -u)" | grep '^fastread-e2e-'); do
    tmux -L "$s" ls >/dev/null 2>&1 && { echo "tmux server $s still running"; exit 1; }
  done
fi
! pgrep -u "$(id -u)" -x fastread >/dev/null || { echo "fastread process left running"; exit 1; }
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || { echo "real state dir created"; exit 1; }
CMD
finish
