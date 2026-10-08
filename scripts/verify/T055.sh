#!/usr/bin/env bash
# Verification for T055 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "e2e files carry the e2e build tag, are test files, vet clean" <<'CMD'
ls e2e/*.go >/dev/null 2>&1 && test -f e2e/resume_test.go || exit 1
for f in e2e/*.go; do
  case "$f" in *_test.go) ;; *) echo "$f is not a _test.go file"; exit 1;; esac
  head -1 "$f" | grep -qx '//go:build e2e' || { echo "$f lacks //go:build e2e"; exit 1; }
done
go vet -tags e2e ./e2e/
CMD
expect_ok "TestE2ESIGTERMSavesState, TestE2EResume, TestE2ERestore pass; no leftovers; real state dir untouched" 300 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
out="$(go test -tags e2e -count=1 -timeout 240s -v -run '^TestE2E(SIGTERMSavesState|Resume|Restore)$' ./e2e/ 2>&1)"; rc=$?
printf '%s\n' "$out" | tail -n 60
test "$rc" -eq 0 || exit 1
for n in TestE2ESIGTERMSavesState TestE2ESIGTERMSavesState/SIGTERM TestE2ESIGTERMSavesState/SIGINT TestE2EResume TestE2ERestore; do
  printf '%s\n' "$out" | grep -qE -- "^ *--- PASS: $n \(" || { echo "no PASS line for $n"; exit 1; }
done
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
