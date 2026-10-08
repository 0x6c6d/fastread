#!/usr/bin/env bash
# Verification for T023 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestFlags TestHelpVersion TestParseFlags TestRunUsage TestRunStartBeyond TestRunEmptyText; do
expect_ok "$t passes" <<CMD
passes $t cmd/fastread
CMD
done
expect_ok "cmd tests pass with -tags nogui" <<'CMD'
go test -tags nogui -count=1 ./cmd/fastread
CMD
expect_ok "TestFlags covers the AC9 table" <<'CMD'
f=cmd/fastread/cli_test.go
for s in '--wpm 49' '--wpm 1501' '--wpm x' '--size 0' '--size 6' '--ui foo' '--start -1' '--start 9999' '--bogus' 'ui reached'; do
  grep -qF -- "$s" "$f" || { echo "missing row: $s"; exit 1; }
done
CMD
expect_ok "binary: help and version exit 0 on stdout" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
for h in --help -h; do
  "$T/fr" "$h" </dev/null >"$T/out" 2>"$T/err" && grep -q 'Usage:' "$T/out" && test ! -s "$T/err" || exit 1
done
"$T/fr" --version </dev/null >"$T/out" 2>"$T/err" && grep -qE '^fastread [^ ]+$' "$T/out" && test ! -s "$T/err"
CMD
expect_ok "negative: AC9 bad flags, --start 9999, two args -> exit 2 with error line and usage" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
while IFS= read -r a; do
  # shellcheck disable=SC2086
  env XDG_STATE_HOME="$T/state" "$T/fr" $a </dev/null >"$T/out" 2>"$T/err"; rc=$?
  test "$rc" -eq 2 || { echo "'$a' exited $rc, want 2"; cat "$T/err"; exit 1; }
  head -1 "$T/err" | grep -q '^fastread: ' && grep -q 'Usage:' "$T/err" && test ! -s "$T/out" ||
    { echo "'$a': bad stderr/stdout"; exit 1; }
done <<'LIST'
--wpm 49 x
--wpm 1501 x
--wpm x x
--wpm=49 x
--size 0 x
--size 6 x
--ui foo x
--start -1 x
--start 9999 x
--bogus x
a b
x --wpm 100
LIST
test ! -e "$T/state"
CMD
finish
