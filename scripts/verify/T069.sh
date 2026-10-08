#!/usr/bin/env bash
# Verification for T069 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Final gate: Plan.md "Global Definition of Done" D1-D17, one labelled check (or more) per D
# line, in order; also the Phase 7 exit criterion. Exit 0 only if every line passes.
# D16's report material (TestE2EInputTypes CASE lines, Wayland and pixel-check status) is
# printed by the D16 checks; the supervisor pastes this script's whole output into the final
# loop summary.
. "$(dirname "$0")/_lib.sh"
DOD_TMP="$(mktemp -d)"
export DOD_TMP
trap 'rm -rf "$DOD_TMP"' EXIT
test -e "$HOME/.local/state/fastread" && touch "$DOD_TMP/state-pre"

expect_ok "D0 every other Tasks.md row is done or superseded" <<'CMD'
awk -F'|' '/^\| T[0-9]+ /{id=$2; st=$7; gsub(/ /,"",id); gsub(/ /,"",st); if (id!="T069" && st!="done" && st!="superseded") {print id" "st; bad=1}} END{exit bad}' Tasks.md
CMD
expect_ok "D1 check (Plan check command, verbatim)" 2400 <<'CMD'
git status --porcelain >"$DOD_TMP/d1.status" && git rev-parse HEAD >"$DOD_TMP/d1.head.pre" || exit 1
test -z "$(gofmt -l .)" && go vet ./... && go vet -tags nogui ./... && go vet -tags e2e ./... && go test ./... && go test -race ./... && go test -tags nogui ./... && go build ./... && go build -tags nogui ./... && CGO_ENABLED=0 go build -tags nogui ./... &&
mv "$DOD_TMP/d1.head.pre" "$DOD_TMP/d1.head"
CMD
expect_ok "D2 toolchain (verbatim)" 600 <<'CMD'
GOTOOLCHAIN=local go build ./... && grep -qE '^go 1\.24(\.[0-9]+)?$' go.mod && grep -q 'gioui.org v0.10.3' go.mod
CMD
expect_ok "D3 input R1-R7: every Phase 2 internal/input test passes" 900 <<'CMD'
for t in TestSelectSource TestDetectType TestMarkdownStrip TestEPUBSpineOrder TestEPUBCorrupt TestEPUBLimits TestEPUBEncrypted TestFB2BodyOnly TestPDFText TestPDFNoText TestPDFCorrupt TestUnsupportedMismatch; do passes $t internal/input || { echo "D3: $t"; exit 1; }; done
CMD
expect_ok "D4 CLI R11/R12/perf: the six cmd/fastread tests pass" 900 <<'CMD'
for t in TestFlags TestHelpVersion TestExitCodes TestErrorsIsChain TestPerfLoadTokenize5MB TestRunRawNoState; do passes $t cmd/fastread || { echo "D4: $t"; exit 1; }; done
CMD
expect_ok "D4 CLI rows on the binary: each invalid flag exits 2 with one fastread: line plus usage on stderr; help/version exit 0 on stdout" 300 <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
for a in '--wpm 49' '--wpm 1501' '--wpm x' '--size 0' '--size 6' '--ui foo' '--start -1' '--start 9999' '--bogus'; do
  env XDG_STATE_HOME="$T/state" "$T/fr" $a 'a b c' </dev/null >"$T/out" 2>"$T/err"; rc=$?
  echo "fastread $a 'a b c': exit=$rc $(head -1 "$T/err")"
  test "$rc" -eq 2 && test ! -s "$T/out" && head -1 "$T/err" | grep -q '^fastread: ' && grep -q 'Usage:' "$T/err" || exit 1
done
for a in --help -h --version; do
  "$T/fr" $a </dev/null >"$T/out" 2>"$T/err"; rc=$?
  echo "fastread $a: exit=$rc $(head -1 "$T/out")"
  test "$rc" -eq 0 && test -s "$T/out" && test ! -s "$T/err" || exit 1
done
grep -qE '^fastread [^ ]+$' "$T/out" && test ! -e "$T/state"
CMD
expect_ok "D5 core R8-R10/R26/R29: every Phase 3 test passes" 900 <<'CMD'
passes TestTokenize internal/tokenize && passes TestSanitize internal/tokenize && passes TestPosition internal/orp && passes TestIndex internal/orp && passes TestDelay internal/timing && passes TestDelayPara internal/timing && for t in TestPlayerKeys TestEffectiveWPM TestScheduleNoDrift TestStartIndex TestResumeRoundTrip TestResumeAtomic TestResumeHashMismatch TestResumeDeleteAtEnd TestResumeModes TestResumeCorruptIgnored TestResumeDirFallback; do passes $t internal/state || { echo "D5: $t"; exit 1; }; done && passes TestRunRawNoState cmd/fastread
CMD
expect_ok "D5 AC7/AC8 values checked directly on orp and timing (scripts/verify/_dodcheck.go)" 300 <<'CMD'
d="$(mktemp -d ./.dodcheck.XXXXXX)" || exit 1; trap 'rm -rf "$d"' EXIT
cp scripts/verify/_dodcheck.go "$d/main.go" && go run "$d/main.go"
CMD
expect_ok "D6 TUI render: every Phase 4 test passes, BenchmarkRenderFrame runs" 900 <<'CMD'
for t in TestGolden TestGlyphFallback TestFocusColumn TestColorModes TestNoColor TestSizeFallback TestTooSmall TestWideAndCombining TestTicks TestSplitLongWord TestProgressHelpFocus TestNoProgressFrame TestEncodeStripsControls TestRenderFast; do passes $t internal/tui || { echo "D6: $t"; exit 1; }; done && go test -run '^$' -bench '^BenchmarkRenderFrame$' -benchtime 100x ./internal/tui/ | grep -q '^BenchmarkRenderFrame'
CMD
expect_ok "D6 coverage: goldens for a/Hello/wonderful/naive x sizes 1-5 plus paused/progress/help frames; TestFocusColumn word classes and widths" <<'CMD'
g=internal/tui/testdata/golden
for w in a hello wonderful naive; do for n in 1 2 3 4 5; do
  test -s "$g/${w}_s$n.golden" || { echo "missing golden ${w}_s$n"; exit 1; }
done; done
for n in hello_s3_paused hello_s2_progress_off hello_s2_help_on; do test -s "$g/$n.golden" || { echo "missing golden $n"; exit 1; }; done
f=internal/tui/split_test.go; test -f "$f" || exit 1
for w in 'naïve' '日本語の本' '👍' 'extraordinary' '1,000,000' '121'; do grep -qF -- "$w" "$f" || { echo "TestFocusColumn lacks: $w"; exit 1; }; done
CMD
expect_ok "D7 TUI runtime: every Phase 5 internal/tui test passes" 900 <<'CMD'
for t in TestKeyDecode TestRestoreOnPanic TestLoopNoLeak TestLoopDriftRealClock TestLoopResize; do passes $t internal/tui || { echo "D7: $t"; exit 1; }; done
CMD
expect_ok "D8 GUI layout: every Phase 6 internal/gui test passes; TestLayoutFocusXAllSizes >= 30 words x 5 levels x widths 200,600,601,1920" 900 <<'CMD'
for t in TestLayoutFocusX TestLayoutBaseline TestLayoutTicks TestLayoutShrinkSplit TestLayoutFocusXAllSizes TestGUIKeyMap; do passes $t internal/gui || { echo "D8: $t"; exit 1; }; done
out="$(go test -count=1 -v -run '^TestLayoutFocusXAllSizes$' ./internal/gui 2>&1)" || exit 1
line="$(printf '%s\n' "$out" | grep -oE 'words=[0-9]+ levels=5 widths=200,600,601,1920 cases=[0-9]+' | head -1)"
echo "$line"; test -n "$line" && test "$(printf '%s\n' "$line" | sed 's/^words=\([0-9]*\).*/\1/')" -ge 30
CMD
expect_ok "D9 e2e: the whole e2e suite passes with every named --- PASS line; nothing left running" 1200 <<'CMD'
xv0="$(pgrep -u "$(id -u)" -x Xvfb | sort)"
out="$(go test -tags e2e -count=1 -timeout 600s -v ./e2e/ 2>&1)"; rc=$?
printf '%s\n' "$out" | grep -E -e '^--- ' -e '^(ok|FAIL)'
test "$rc" -eq 0 || { printf '%s\n' "$out" | tail -n 60; exit 1; }
for n in TestE2EBasic TestE2EStdin TestE2ENotFound TestE2ENoArgTTY TestE2ERestore TestE2EFocusColumn TestE2EResize TestE2ESIGTERMSavesState TestE2EResume TestE2EGUIXvfb TestE2EGUINoDisplay TestE2EGUIBadDisplay TestE2ENoguiBinary TestE2EInputTypes; do
  printf '%s\n' "$out" | grep -qE -- "^--- PASS: $n \(" || { echo "D9: no PASS line for $n"; exit 1; }
done
sleep 0.5
test "$(pgrep -u "$(id -u)" -x Xvfb | sort)" = "$xv0" || { echo "an Xvfb started by the tests is still running"; exit 1; }
! pgrep -u "$(id -u)" -x fastread >/dev/null || { echo "fastread process left running"; exit 1; }
if ls "/tmp/tmux-$(id -u)" 2>/dev/null | grep -q '^fastread-e2e-'; then
  for s in $(ls "/tmp/tmux-$(id -u)" | grep '^fastread-e2e-'); do
    tmux -L "$s" ls >/dev/null 2>&1 && { echo "tmux server $s still running"; exit 1; }
  done
fi
exit 0
CMD
expect_ok "D10 layering R30 (verbatim)" <<'CMD'
test -z "$(go list -deps ./internal/input ./internal/tokenize ./internal/orp ./internal/timing ./internal/state | grep -e fastread/internal/tui -e fastread/internal/gui -e gioui.org)" && test -z "$(grep -rl gioui.org --include='*.go' . | grep -v '^./internal/gui/')"
CMD
expect_ok "D11 nogui binary R31 (verbatim)" 300 <<'CMD'
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
CGO_ENABLED=0 go build -tags nogui -o $T/fr ./cmd/fastread && ! go version -m $T/fr | grep -q gioui.org && ! go tool nm $T/fr | grep -q gioui.org && { env -u DISPLAY -u WAYLAND_DISPLAY $T/fr --ui gui x 2>$T/err; test $? -eq 1; } && test "$(wc -l < $T/err)" -eq 1
CMD
expect_ok "D12 no-display GUI R24 (verbatim)" 300 <<'CMD'
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
go build -o $T/fg ./cmd/fastread && { env -u DISPLAY -u WAYLAND_DISPLAY $T/fg --ui gui x 2>$T/err2; test $? -eq 1; } && test "$(wc -l < $T/err2)" -eq 1
CMD
expect_ok "D13 protected files (verbatim)" <<'CMD'
scripts/check-protected.sh
CMD
expect_ok "D14 deliverables (verbatim), licence checks of T012/T066, path-like rule documented" <<'CMD'
head -1 LICENSE | grep -qx 'MIT License' && grep -q 'Lucas Menke' LICENSE && test -s third_party/gofont/LICENSE && test -s third_party/misc-fixed/LICENSE && for h in '## Install and build' '## Examples' '## Flags' '## Keys' '## Dependencies' '## GUI font' '## Limitations'; do grep -qF "$h" docs/usage.md || exit 1; done && for x in stdin .txt .md .epub .fb2 .pdf; do grep -qF -- "$x" docs/usage.md || exit 1; done &&
bash scripts/verify/T012.sh >/dev/null && bash scripts/verify/T066.sh >/dev/null &&
awk -v RS= '/path-like/ && /whitespace/ && /not found/ {ok=1} END {exit !ok}' docs/usage.md
CMD
expect_ok "D15 privacy: no network code in cmd/ or internal/ (verbatim); real ~/.local/state/fastread not created by the run" <<'CMD'
test -z "$(grep -rln --include='*.go' -e 'net/http' -e '"net"' cmd internal)" || exit 1
test -e "$DOD_TMP/state-pre" || test ! -e "$HOME/.local/state/fastread" || { echo "the real state dir was created during this run"; exit 1; }
CMD
expect_ok "D16 report: TestE2EInputTypes real output and exit codes (raw, stdin, txt, md, epub, fb2, pdf, missing file, empty text, bad flag, scanned PDF, wrong extension)" 600 <<'CMD'
out="$(go test -tags e2e -count=1 -timeout 300s -v -run '^TestE2EInputTypes$' ./e2e/ 2>&1)"; rc=$?
printf '%s\n' "$out" | grep -oE 'CASE name=.*'
test "$rc" -eq 0 || { printf '%s\n' "$out" | tail -n 40; exit 1; }
for c in 'raw exit=0' 'stdin exit=0' 'txt exit=0' 'md exit=0' 'epub exit=0' 'fb2 exit=0' 'pdf exit=0' 'missing-file exit=1' 'empty-text exit=1' 'bad-flag exit=2' 'scanned-pdf exit=1' 'wrong-extension exit=1'; do
  printf '%s\n' "$out" | grep -qF -- "CASE name=$c output=" || { echo "D16: no line 'CASE name=$c'"; exit 1; }
done
printf '%s\n' "$out" | grep -F 'CASE name=scanned-pdf' | grep -qF 'no extractable text' &&
printf '%s\n' "$out" | grep -F 'CASE name=missing-file' | grep -qF 'not found' &&
printf '%s\n' "$out" | grep -F 'CASE name=empty-text' | grep -qF 'empty text'
CMD
expect_ok "D16 report: Wayland status" <<'CMD'
echo "Wayland: not run (WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-unset}; no headless compositor, the user's own session is off limits; X11 verified under Xvfb in D9)"
CMD
expect_ok "D16 report: optional AC19 pixel check on Xvfb (xwd), or 'not run'" 300 <<'CMD'
command -v xwd >/dev/null || { echo "pixel check: not run (xwd absent; import absent)"; exit 0; }
. scripts/verify/_gui.sh; gui_setup || exit 1
words="$(for i in $(seq 60); do printf 'hello '; done)"
gui_start a --ui gui --no-resume --wpm 50 "$words" || exit 1
gui_frame p1 600 300 || exit 1
DISPLAY="$D" timeout 5 xdotool windowsize --sync "$WID" 1280 720 >/dev/null 2>&1
gui_frame p2 1280 720 || exit 1
gui_keys q; gui_wait 3 && test "$RC" -eq 0 && test ! -s "$T/a.err" || exit 1
echo "pixel check: run (xwd): red focus box centred on W/2 and ticks at W/2-1..W/2 at 600x300 and 1280x720; q exits 0"
CMD
expect_ok "D17 final: D1 passed on the final commit (HEAD unchanged since D1, no uncommitted code while D1 ran)" <<'CMD'
test -s "$DOD_TMP/d1.head" || { echo "D1 did not pass"; exit 1; }
test "$(git rev-parse HEAD)" = "$(cat "$DOD_TMP/d1.head")" || { echo "HEAD moved after D1"; exit 1; }
if grep -v -e ' Tasks\.md$' -e ' Improvements\.md$' -e ' \.README\.md\.swp$' "$DOD_TMP/d1.status"; then
  echo "uncommitted changes (above) while D1 ran: D1 did not test the final commit"; exit 1
fi
exit 0
CMD
expect_fail "negative: a missing path-like file is an error" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 125
cd "$T" && env XDG_STATE_HOME="$T/state" "$T/fr" ./nope-dod.txt </dev/null
CMD
expect_fail "negative: --wpm 49 is a usage error" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 125
env XDG_STATE_HOME="$T/state" "$T/fr" --wpm 49 hello </dev/null
CMD
expect_fail "negative: a text-less (scanned) PDF is an error" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 125
env XDG_STATE_HOME="$T/state" "$T/fr" internal/input/testdata/notext.pdf </dev/null
CMD
finish
