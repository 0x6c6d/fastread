#!/usr/bin/env bash
# Verification for T024 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestErrorsIsChain TestExitCodes TestRunEmptyText TestRunNoInputTTY TestRunStartBeyond TestRunTerminalError TestRunTUIPlaysToEnd TestRunGUINoDisplay TestRunUsage; do
expect_ok "$t passes" <<CMD
passes $t cmd/fastread
CMD
done
expect_ok "cmd tests pass with -tags nogui and -race" <<'CMD'
go test -tags nogui -count=1 ./cmd/fastread && go test -race -count=1 ./cmd/fastread
CMD
expect_ok "declarations in run.go" <<'CMD'
f=cmd/fastread/run.go
grep -qF 'type usageError struct' "$f" &&
grep -qF 'func prepare(o options, stdin io.Reader, isTTY bool) (input.Document, []tokenize.Token, error)' "$f" &&
grep -qF 'func exitCode(err error) int' "$f" &&
grep -qF 'func runErr(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) (int, error)' "$f"
CMD
expect_fail "negative: run.go or internal/input wraps an error with %v only (no %w)" <<'CMD'
grep -nE 'Errorf\(.*\berr\)' cmd/fastread/run.go $(ls internal/input/*.go | grep -v -e '_test\.go$' -e 'pdf\.go$') |
  grep -F '%v' | grep -vF '%w'
CMD
expect_ok "negative: runtime errors exit 1 with one line naming the error" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
e=internal/input/testdata/sample.epub
head -c $(( $(wc -c < "$e") / 2 )) "$e" > "$T/half.epub"
printf 'hello world\n' > "$T/x.pdf"
printf '   \n\n' > "$T/blank.txt"
while IFS='|' read -r arg want; do
  env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$arg" </dev/null >"$T/out" 2>"$T/err"; rc=$?
  test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && grep -q "^fastread: .*$want" "$T/err" && test ! -s "$T/out" ||
    { echo "$arg: rc=$rc want 1 and '$want'"; cat "$T/err"; exit 1; }
done <<LIST
$T/nope.txt|file not found
$T/half.epub|corrupt EPUB
$T/blank.txt|empty text
$T/x.pdf|unsupported file type
internal/input/testdata/notext.pdf|no extractable text (scanned PDF?)
LIST
test ! -e "$T/state"
CMD
expect_ok "negative: usage errors exit 2 with usage on stderr" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
"$T/fr" --start 5 "a b" </dev/null >"$T/out" 2>"$T/err"; rc=$?
test "$rc" -eq 2 && head -1 "$T/err" | grep -q '^fastread: ' && grep -q 'Usage:' "$T/err" && test ! -s "$T/out" || exit 1
script -qec "$T/fr" /dev/null </dev/null >"$T/out2" 2>&1; rc=$?
test "$rc" -eq 2 && grep -q 'Usage:' "$T/out2"
CMD
finish
