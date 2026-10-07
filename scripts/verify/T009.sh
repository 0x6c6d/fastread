#!/usr/bin/env bash
# Verification for T009 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestParseFlags passes" <<'CMD'
passes TestParseFlags cmd/fastread
CMD
expect_ok "TestRunUsage passes" <<'CMD'
passes TestRunUsage cmd/fastread
CMD
expect_ok "binary: --help and -h print usage to stdout, exit 0" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
for h in --help -h; do
  "$T/fr" "$h" >"$T/out" 2>"$T/err" || exit 1
  grep -q 'Usage:' "$T/out" && test ! -s "$T/err" || exit 1
  for f in --wpm --size --ui --start --no-resume --no-progress --version --help; do
    grep -qF -- "$f" "$T/out" || { echo "usage lacks $f"; exit 1; }
  done
done
CMD
expect_ok "binary: --version honours -ldflags -X main.version" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -ldflags '-X main.version=1.2.3' -o "$T/fr" ./cmd/fastread || exit 1
test "$("$T/fr" --version)" = 'fastread 1.2.3'
CMD
expect_ok "negative: invalid flags exit 2 with error line and usage on stderr" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
while IFS= read -r a; do
  # shellcheck disable=SC2086
  "$T/fr" $a </dev/null >"$T/out" 2>"$T/err"; rc=$?
  test "$rc" -eq 2 || { echo "'$a' exited $rc, want 2"; exit 1; }
  head -1 "$T/err" | grep -q '^fastread: ' || { echo "'$a' no error line"; exit 1; }
  grep -q 'Usage:' "$T/err" || { echo "'$a' no usage on stderr"; exit 1; }
  test ! -s "$T/out" || { echo "'$a' wrote to stdout"; exit 1; }
done <<'LIST'
--wpm 49 x
--wpm 1501 x
--wpm x x
--size 0 x
--size 6 x
--ui foo x
--start -1 x
--bogus x
a b
hello --wpm 100
LIST
CMD
expect_fail "negative: os.Exit outside main()" <<'CMD'
ls cmd/fastread/*.go >/dev/null 2>&1 || exit 0
grep -n 'os\.Exit' cmd/fastread/*.go | grep -v -e 'main.go:.*os\.Exit(run(' -e '_test\.go:'
CMD
finish
