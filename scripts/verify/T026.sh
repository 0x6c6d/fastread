#!/usr/bin/env bash
# Verification for T026 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Also the Phase 2 exit criterion of Plan.md.
. "$(dirname "$0")/_lib.sh"
expect_ok "Phase 2 exit criterion (check + named tests)" 1500 <<'CMD'
check &&
for t in TestSelectSource TestDetectType TestMarkdownStrip TestEPUBSpineOrder TestEPUBCorrupt TestEPUBLimits TestEPUBEncrypted TestFB2BodyOnly TestPDFText TestPDFNoText TestPDFCorrupt TestUnsupportedMismatch; do passes $t internal/input || exit 1; done &&
for t in TestFlags TestHelpVersion TestExitCodes TestErrorsIsChain TestPerfLoadTokenize5MB; do passes $t cmd/fastread || exit 1; done
CMD
expect_ok "TestPrepareFixtures passes (also -tags nogui)" <<'CMD'
passes TestPrepareFixtures cmd/fastread &&
go test -tags nogui -count=1 -run '^TestPrepareFixtures$' ./cmd/fastread
CMD
expect_ok "sample.txt fixture content exact" <<'CMD'
test "$(od -An -c internal/input/testdata/sample.txt | tr -s ' ')" = "$(printf 'Plain text sample.\n\nSecond paragraph here.\n' | od -An -c | tr -s ' ')"
CMD
expect_ok "protected files unchanged" <<'CMD'
scripts/check-protected.sh
CMD
expect_ok "binary loads every fixture (fails only for lack of a terminal)" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
for f in sample.txt sample.md sample.epub sample.fb2 sample.pdf; do
  setsid -w env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "internal/input/testdata/$f" </dev/null >"$T/out" 2>"$T/err"; rc=$?
  echo "$f: rc=$rc $(head -1 "$T/err")"
  test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 &&
    ! grep -qi -e 'corrupt' -e 'unsupported' -e 'empty' -e 'not found' -e 'no extractable' -e 'too large' "$T/err" || exit 1
done
CMD
expect_ok "negative: notext.pdf and extension mismatches exit 1 with the typed error" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
d=internal/input/testdata
env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$d/notext.pdf" </dev/null 2>"$T/err"; rc=$?
test "$rc" -eq 1 && grep -q 'no extractable text' "$T/err" || { cat "$T/err"; exit 1; }
cp "$d/sample.epub" "$T/book.pdf"; cp "$d/sample.pdf" "$T/book.fb2"; cp "$d/sample.txt" "$T/book.epub"
for f in book.pdf book.fb2 book.epub; do
  env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/$f" </dev/null 2>"$T/err"; rc=$?
  test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && grep -q 'unsupported' "$T/err" || { echo "$f rc=$rc"; cat "$T/err"; exit 1; }
done
test ! -e "$T/state"
CMD
finish
