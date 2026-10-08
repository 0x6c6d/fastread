#!/usr/bin/env bash
# Verification for T044 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestGolden passes" <<'CMD'
passes TestGolden internal/tui
CMD
expect_ok "all 28 golden files present" <<'CMD'
g=internal/tui/testdata/golden
for w in a hello wonderful naive; do for n in 1 2 3 4 5; do
  test -s "$g/${w}_s$n.golden" || { echo "missing ${w}_s$n"; exit 1; }
done; done
for n in hello_s3_paused hello_s2_progress_off hello_s2_help_on naive_s1_help_on_progress_off \
  wonderful_s5_part1 nihon_s3_fallback hello_s5_40x8 toosmall_19x4; do
  test -s "$g/$n.golden" || { echo "missing $n"; exit 1; }
done
CMD
expect_ok "pinned headers and spot contents" <<'CMD'
g=internal/tui/testdata/golden
hd() { head -1 "$g/$1.golden"; }
hd hello_s3 | grep -q 'w=80 h=24 size=3 parts=1' &&
hd wonderful_s3 | grep -q 'parts=1' && hd wonderful_s4 | grep -q 'parts=2' &&
hd wonderful_s5 | grep -q 'size=5 parts=2' && hd nihon_s3_fallback | grep -q 'size=1' &&
hd hello_s5_40x8 | grep -q 'w=40 h=8 size=2' && hd toosmall_19x4 | grep -q 'size=0 parts=0' &&
grep -qF '█▀▀▀█' "$g/hello_s3.golden" && grep -qF 'naïve' "$g/naive_s1.golden" &&
grep -qF 'terminal too small' "$g/toosmall_19x4.golden" &&
grep -qF 'word 5/120  287 wpm' "$g/hello_s3.golden" &&
grep -qxF -- '--' "$g/hello_s3.golden" && grep -q 'T' "$g/hello_s3_paused.golden" &&
! grep -qF 'word 5/120' "$g/hello_s2_progress_off.golden" &&
grep -qF 'space pause' "$g/hello_s2_help_on.golden"
CMD
expect_fail "negative: TestGolden fails on a corrupted golden (restored afterwards)" 300 <<'CMD'
f=internal/tui/testdata/golden/hello_s3.golden; test -s "$f" || exit 0
b="$(mktemp)"; cp "$f" "$b" || exit 0; trap 'cp "$b" "$f"; rm -f "$b"' EXIT
sed -i '5s/.$/#/' "$f"
go test -count=1 -run '^TestGolden$' ./internal/tui
CMD
expect_ok "golden file restored and still passing" <<'CMD'
go test -count=1 -run '^TestGolden$' ./internal/tui
CMD
finish
