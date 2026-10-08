#!/usr/bin/env bash
# Verification for T036 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Also the Phase 3 exit criterion of Plan.md.
. "$(dirname "$0")/_lib.sh"
expect_ok "Phase 3 exit criterion (check + named tests); real state dir untouched" 1500 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
check && passes TestTokenize internal/tokenize && passes TestSanitize internal/tokenize &&
passes TestPosition internal/orp && passes TestIndex internal/orp &&
passes TestDelay internal/timing && passes TestDelayPara internal/timing &&
for t in TestPlayerKeys TestEffectiveWPM TestScheduleNoDrift TestStartIndex TestResumeRoundTrip TestResumeAtomic TestResumeHashMismatch TestResumeDeleteAtEnd TestResumeModes TestResumeCorruptIgnored TestResumeDirFallback; do passes $t internal/state || exit 1; done &&
passes TestRunRawNoState cmd/fastread || exit 1
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread"
CMD
expect_ok "TestPrepareSanitized and TestCorePipeline pass (also -tags nogui)" <<'CMD'
passes TestPrepareSanitized cmd/fastread && passes TestCorePipeline cmd/fastread &&
go test -tags nogui -count=1 -run '^(TestPrepareSanitized|TestCorePipeline)$' ./cmd/fastread
CMD
expect_ok "TestCorePipeline covers every committed fixture" <<'CMD'
out="$(go test -count=1 -v -run '^TestCorePipeline$' ./cmd/fastread 2>&1)" || { printf '%s\n' "$out" | tail -20; exit 1; }
test "$(printf '%s\n' "$out" | grep -c -- '--- PASS: TestCorePipeline/')" -ge 5
CMD
expect_ok "protected files unchanged" <<'CMD'
scripts/check-protected.sh
CMD
expect_fail "negative: logic packages import UI, Gio or network packages" <<'CMD'
out="$(go list -deps ./internal/input ./internal/tokenize ./internal/orp ./internal/timing ./internal/state)" || exit 0
printf '%s\n' "$out" | grep -q -x -e 'net' -e 'net/http' -e 'github.com/0x6c6d/fastread/internal/tui' \
  -e 'github.com/0x6c6d/fastread/internal/gui' -e 'gioui.org.*'
CMD
expect_fail "negative: network code or gioui.org outside internal/gui" <<'CMD'
grep -rln --include='*.go' -e 'net/http' -e '"net"' cmd internal && exit 0
grep -rl gioui.org --include='*.go' . | grep -v '^./internal/gui/'
CMD
finish
