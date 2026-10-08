#!/usr/bin/env bash
# Verification for T014 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestSelectSource TestLooksLikePath TestSelectBasic; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "declarations in select.go, Select removed from input.go" <<'CMD'
grep -qF 'func Select(arg string, hasArg, stdinIsTTY bool) (Source, error)' internal/input/select.go &&
grep -qF 'func LooksLikePath(arg string) bool' internal/input/select.go &&
! grep -q '^func Select(' internal/input/input.go
CMD
expect_ok "negative: path-like missing arguments exit 1 with one 'not found' line" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
cd "$T" || exit 1
for a in ./nope.txt nope.epub NOPE.PDF '~/definitely-missing' sub/missing.md; do
  env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$a" </dev/null >"$T/out" 2>"$T/err"; rc=$?
  test "$rc" -eq 1 || { echo "$a: exit $rc, want 1"; cat "$T/err"; exit 1; }
  test "$(wc -l < "$T/err")" -eq 1 || { echo "$a: stderr not one line"; cat "$T/err"; exit 1; }
  grep -q '^fastread: .*not found' "$T/err" || { echo "$a: no 'not found'"; cat "$T/err"; exit 1; }
  test ! -s "$T/out" || exit 1
done
test ! -e "$T/state" || { echo "state dir created for a missing file"; exit 1; }
CMD
expect_ok "negative: a FIFO argument exits 1 promptly (never opened)" 30 <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
mkfifo "$T/pipe.txt" || exit 1
timeout 5 env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/pipe.txt" </dev/null 2>"$T/err"; rc=$?
cat "$T/err"
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1
CMD
expect_ok "negative: a directory argument exits 1 with one line" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
"$T/fr" --no-resume "$T" </dev/null 2>"$T/err"; rc=$?
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1
CMD
finish
