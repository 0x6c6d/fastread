#!/usr/bin/env bash
# Verification for T025 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestPerfLoadTokenize5MB passes" <<'CMD'
passes TestPerfLoadTokenize5MB cmd/fastread
CMD
expect_ok "TestPerfLoadTokenize5MB passes under -race and -tags nogui, both rows run" 600 <<'CMD'
for tags in "-race" "-tags nogui"; do
  # shellcheck disable=SC2086
  out="$(go test $tags -count=1 -v -run '^TestPerfLoadTokenize5MB$' ./cmd/fastread 2>&1)" || { printf '%s\n' "$out" | tail -20; exit 1; }
  printf '%s\n' "$out" | grep -q -- '--- PASS: TestPerfLoadTokenize5MB/file' &&
  printf '%s\n' "$out" | grep -q -- '--- PASS: TestPerfLoadTokenize5MB/stdin' || { printf '%s\n' "$out" | tail -20; exit 1; }
done
CMD
expect_ok "race constant files and 5 MiB / 2 s bound present" <<'CMD'
grep -q '^//go:build race$' cmd/fastread/race_on_test.go && grep -q 'raceEnabled = true' cmd/fastread/race_on_test.go &&
grep -q '^//go:build !race$' cmd/fastread/race_off_test.go && grep -q 'raceEnabled = false' cmd/fastread/race_off_test.go &&
grep -qE '5 *<< *20' cmd/fastread/perf_test.go && grep -qE '2 *\* *time\.Second' cmd/fastread/perf_test.go
CMD
expect_fail "negative: the perf test can be skipped" <<'CMD'
grep -nE -e 't\.Skip' -e 'testing\.Short' cmd/fastread/perf_test.go
CMD
finish
