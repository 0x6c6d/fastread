#!/usr/bin/env bash
# Verification for T032 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestResumeDirFallback TestResumeModes TestResumeAtomic TestResumeDeleteAtEnd; do
expect_ok "$t passes" <<CMD
passes $t internal/state
CMD
done
expect_ok "store tests pass under -race and repeated; real state dir untouched" 400 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
go test -race -count=3 -run '^TestResume' ./internal/state || exit 1
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread"
CMD
expect_ok "store API declared" <<'CMD'
d="$(go doc -all ./internal/state)" || exit 1
for s in 'ErrNoStateDir' 'func Dir\(getenv func\(string\) string\) \(string, error\)' \
  'func NewStore\(dir string\) \*Store' 'func \([a-z]+ \*Store\) EntryPath\(path string\) string' \
  'func \([a-z]+ \*Store\) Save\(path string, sum \[32\]byte, index int\) error' \
  'func \([a-z]+ \*Store\) Delete\(path string\) error' 'func \([a-z]+ \*Store\) Dir\(\) string'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
grep -qE 'writeData +=' internal/state/store.go && grep -qE 'renameFile +=' internal/state/store.go &&
grep -q 'os.CreateTemp' internal/state/store.go
CMD
expect_fail "negative: non-atomic write or fixed temp name in store.go" <<'CMD'
grep -nE -e 'os\.WriteFile' -e 'ioutil\.' -e 'os\.Create\(' -e 'os\.OpenFile\(' internal/state/store.go
CMD
expect_fail "negative: internal/state has a UI, Gio or network dependency" <<'CMD'
out="$(go list -deps ./internal/state)" || exit 0
printf '%s\n' "$out" | grep -q -x -e 'net' -e 'net/http' -e 'github.com/0x6c6d/fastread/internal/tui' \
  -e 'github.com/0x6c6d/fastread/internal/gui' -e 'gioui.org.*'
CMD
finish
