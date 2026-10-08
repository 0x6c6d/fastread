#!/usr/bin/env bash
# Verification for T067 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check passes" 1800 <<'CMD'
check
CMD
expect_ok "e2e files tagged, vet clean; inputtypes_test.go exists" <<'CMD'
test -f e2e/inputtypes_test.go || exit 1
for f in e2e/*.go; do
  case "$f" in *_test.go) ;; *) echo "$f is not a _test.go file"; exit 1;; esac
  head -1 "$f" | grep -qx '//go:build e2e' || { echo "$f lacks //go:build e2e"; exit 1; }
done
go vet -tags e2e ./e2e/
CMD
expect_ok "TestE2EInputTypes passes and logs all twelve CASE lines with the expected codes and outputs; no leftovers; real state dir untouched" 600 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
out="$(go test -tags e2e -count=1 -timeout 300s -v -run '^TestE2EInputTypes$' ./e2e/ 2>&1)"; rc=$?
printf '%s\n' "$out" | grep -e 'CASE name=' -e '^--- ' -e '^    --- ' -e '^FAIL' -e '^ok'
test "$rc" -eq 0 || { printf '%s\n' "$out" | tail -n 40; exit 1; }
printf '%s\n' "$out" | grep -qE -- '^--- PASS: TestE2EInputTypes \(' || { echo "no PASS line"; exit 1; }
want() { printf '%s\n' "$out" | grep -qF -- "CASE name=$1 exit=$2 output=$3" || { echo "missing or wrong: CASE name=$1 exit=$2 output=$3"; exit 1; }; }
want raw 0 '"Hello wonderful world"'
want stdin 0 '"one two three"'
want txt 0 '"Plain text sample. Second paragraph here."'
want md 0 '"Title Here Some emphasis, strong and under text with snake_case kept. A link text and alt words here. Use now, see there. quoted line item one item two item three Setext Heading a b c d Escaped 5 * 3 and html plus ref link too."'
want epub 0 '"One first & café Two second Three third"'
want fb2 0 '"Body Title First body paragraph. Second body paragraph. Note text."'
want pdf 0 '"Hello PDF world. Second page here."'
want missing-file 1 '"fastread: '
want empty-text 1 '"fastread: '
want bad-flag 2 '"fastread: '
want scanned-pdf 1 '"fastread: '
want wrong-extension 1 '"fastread: '
printf '%s\n' "$out" | grep -F 'CASE name=scanned-pdf' | grep -qF 'no extractable text (scanned PDF?)' || exit 1
printf '%s\n' "$out" | grep -F 'CASE name=bad-flag' | grep -qF 'Usage:' || exit 1
test "$(printf '%s\n' "$out" | grep -c 'CASE name=')" -eq 12 || { echo "expected exactly 12 CASE lines"; exit 1; }
sleep 0.5
if ls "/tmp/tmux-$(id -u)" 2>/dev/null | grep -q '^fastread-e2e-'; then
  for s in $(ls "/tmp/tmux-$(id -u)" | grep '^fastread-e2e-'); do
    tmux -L "$s" ls >/dev/null 2>&1 && { echo "tmux server $s still running"; exit 1; }
  done
fi
! pgrep -u "$(id -u)" -x fastread >/dev/null || { echo "fastread process left running"; exit 1; }
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || { echo "real state dir created"; exit 1; }
CMD
expect_ok "independent run of the five error cases: exit code, one stderr line, empty stdout, no state dir" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
d="$PWD/internal/input/testdata"
printf ' \n\n\t\n' >"$T/empty.txt"; cp "$d/sample.epub" "$T/book.pdf"
one() { # <want-code> <want-substring> <lines: 1 or any> args...
  w="$1"; s="$2"; l="$3"; shift 3
  (cd "$T" && env XDG_STATE_HOME="$T/state" "$T/fr" "$@" </dev/null >"$T/out" 2>"$T/err"); rc=$?
  echo "fastread $*: exit=$rc stderr=$(head -1 "$T/err")"
  test "$rc" -eq "$w" && test ! -s "$T/out" && head -1 "$T/err" | grep -q '^fastread: ' &&
    grep -qF -- "$s" "$T/err" && { test "$l" = any || test "$(wc -l <"$T/err")" -eq 1; } &&
    test ! -e "$T/state" || { cat "$T/err"; exit 1; }
}
one 1 'not found' 1 ./nope.txt
one 1 'empty text' 1 "$T/empty.txt"
one 2 'Usage:' any --wpm 49 hello
one 1 'no extractable text (scanned PDF?)' 1 "$d/notext.pdf"
one 1 'unsupported' 1 "$T/book.pdf"
CMD
expect_fail "negative: missing path-like file must fail" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 125
cd "$T" && env XDG_STATE_HOME="$T/state" "$T/fr" ./nope.txt </dev/null
CMD
expect_fail "negative: scanned (text-less) PDF must fail" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 125
env XDG_STATE_HOME="$T/state" "$T/fr" internal/input/testdata/notext.pdf </dev/null
CMD
expect_fail "negative: bad flag must fail" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 125
env XDG_STATE_HOME="$T/state" "$T/fr" --wpm 49 hello </dev/null
CMD
expect_ok "protected files unchanged" <<'CMD'
scripts/check-protected.sh
CMD
finish
