#!/usr/bin/env bash
# Verification for T033 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestResumeRoundTrip TestResumeHashMismatch TestResumeCorruptIgnored TestStartIndex \
  TestResumeDirFallback TestResumeModes TestResumeAtomic TestResumeDeleteAtEnd; do
expect_ok "$t passes" <<CMD
passes $t internal/state
CMD
done
expect_ok "state tests pass under -race; real state dir untouched" 400 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
go test -race -count=2 -run '^(TestResume|TestStartIndex$)' ./internal/state || exit 1
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread"
CMD
expect_ok "declarations" <<'CMD'
d="$(go doc -all ./internal/state)" || exit 1
for s in 'ErrCorruptState' 'func \([a-z]+ \*Store\) Load\(path string, sum \[32\]byte\) \(index int, ok bool, err error\)' \
  'func StartIndex\(start int, startSet, noResume bool, saved int, haveSaved bool, n int\) int'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
grep -qF 'func decodeEntry(b []byte, path string, sum [32]byte) (index int, ok bool, err error)' internal/state/store.go &&
grep -qF 'func FuzzDecodeEntry(f *testing.F)' internal/state/store_load_test.go
CMD
expect_ok "Load uses Lstat, a size limit and DisallowUnknownFields" <<'CMD'
f=internal/state/store.go
grep -q 'os.Lstat' "$f" && grep -q 'io.LimitReader' "$f" && grep -q 'DisallowUnknownFields' "$f"
CMD
expect_ok "negative: FuzzDecodeEntry finds no panic or wrong error class in 15 s" 180 <<'CMD'
go test -count=1 -run '^$' -fuzz '^FuzzDecodeEntry$' -fuzztime 15s ./internal/state
CMD
expect_fail "negative: Load follows symlinks via os.Stat or reads unbounded" <<'CMD'
grep -nE -e 'os\.Stat\(' -e 'os\.ReadFile\(' -e 'io\.ReadAll\(f\)' internal/state/store.go
CMD
finish
