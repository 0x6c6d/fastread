#!/usr/bin/env bash
# Verification for T034 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestRunRawNoState TestRunResumeFile TestRunSignalSaves TestErrorsIsChain TestExitCodes; do
expect_ok "$t passes" <<CMD
passes $t cmd/fastread
CMD
done
expect_ok "cmd tests pass with -tags nogui and -race; real state dir untouched" 600 <<'CMD'
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
go test -tags nogui -count=1 ./cmd/fastread && go test -race -count=1 ./cmd/fastread || exit 1
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread"
CMD
expect_ok "seams, signal context and resumer declared; TODO removed" <<'CMD'
grep -qE 'runTUI +=? *tui\.Run' cmd/fastread/run.go &&
grep -qE 'notifyContext +=? *signal\.NotifyContext' cmd/fastread/run.go &&
grep -q 'syscall.SIGTERM' cmd/fastread/run.go &&
! grep -q 'TODO(phase3)' cmd/fastread/run.go &&
grep -qF 'func newResumer(doc input.Document, getenv func(string) string, stderr io.Writer) *resumer' cmd/fastread/resume.go &&
grep -qF 'func (r *resumer) start(o options, n int, stderr io.Writer) int' cmd/fastread/resume.go &&
grep -qF 'func (r *resumer) finish(p *state.Player, last int) error' cmd/fastread/resume.go
CMD
expect_fail "negative: cmd writes state files itself instead of via state.Store" <<'CMD'
grep -nE -e 'os\.WriteFile' -e 'os\.Create' -e 'os\.MkdirAll' -e 'os\.OpenFile' cmd/fastread/resume.go cmd/fastread/run.go
CMD
expect_ok "binary in tmux: SIGTERM saves, resume, overrides, q saves, end deletes, negatives" 300 <<'CMD'
T="$(mktemp -d)"; S="frv-t034-$$"
pre=0; test -e "$HOME/.local/state/fastread" && pre=1
trap 'tmux -L "$S" kill-server 2>/dev/null; rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
for i in $(seq 0 199); do printf 'w%04d ' "$i"; done > "$T/book.txt"
book="$(realpath "$T/book.txt")"; st="$T/state/fastread"
launch() { # $1 = XDG_STATE_HOME, $2 = arguments (shell-quoted), $3 = optional stdin pipe prefix
  rm -f "$T/rc" "$T/err"
  tmux -L "$S" -f /dev/null new-session -d -s v -x 80 -y 24 \
    "$3 env XDG_STATE_HOME='$1' '$T/fr' $2 2>'$T/err'; echo \$? > '$T/rc'"
}
waitrc() {
  for _ in $(seq 150); do test -s "$T/rc" && break; sleep 0.1; done
  test -s "$T/rc" || { echo "process did not exit"; return 1; }
  for _ in $(seq 30); do tmux -L "$S" has-session -t v 2>/dev/null || return 0; sleep 0.1; done
}
waitword() { # $1 = grep BRE; prints the first match on screen
  for _ in $(seq 80); do
    w="$(tmux -L "$S" capture-pane -p -t v 2>/dev/null | grep -o -- "$1" | head -1)"
    test -n "$w" && { echo "$w"; return 0; }; sleep 0.1
  done
  echo "timeout waiting for $1" >&2; return 1
}
quit() { tmux -L "$S" send-keys -t v q; waitrc; }
idx() { sed -n 's/.*"index": *\([0-9][0-9]*\).*/\1/p' "$1"; }
fail() { echo "FAIL: $*"; test -f "$T/err" && cat "$T/err"; exit 1; }
# 1. SIGTERM saves within 1 s; entry 0600 in 0700 dir, path + index, no text
launch "$T/state" "--size 1 --wpm 50 '$book'"
waitword 'w0002' >/dev/null || fail "word w0002 never shown"
pid="$(pgrep -f "^$T/fr ")"; test -n "$pid" || fail "no pid"
s=$(date +%s%N); kill -TERM "$pid"
for _ in $(seq 50); do kill -0 "$pid" 2>/dev/null || break; sleep 0.02; done
ms=$(( ($(date +%s%N) - s) / 1000000 )); waitrc || exit 1
test "$(cat "$T/rc")" = 0 || fail "SIGTERM exit $(cat "$T/rc")"
test "$ms" -lt 1000 || fail "SIGTERM exit took ${ms} ms"
set -- "$st"/*.json; test "$#" -eq 1 && test -f "$1" || fail "expected exactly one entry"
e="$1"
test "$(stat -c %a "$e")" = 600 && test "$(stat -c %a "$st")" = 700 || fail "modes $(stat -c %a "$e") $(stat -c %a "$st")"
i="$(idx "$e")"; test -n "$i" && test "$i" -ge 2 || fail "index '$i'"
grep -qF "\"$book\"" "$e" || fail "path missing in entry"
! grep -qE 'w0[0-9]{3}' "$e" || fail "entry contains text"
# 2. restart resumes at the saved word; q saves
launch "$T/state" "--size 1 --wpm 50 '$book'"
w="$(waitword 'w0[0-9][0-9][0-9]')" || exit 1
test "$w" = "$(printf 'w%04d' "$i")" || fail "resumed at $w, want index $i"
quit || exit 1; test "$(cat "$T/rc")" = 0 || fail "q exit"
test "$(idx "$e")" -ge "$i" || fail "index after q"
# 3. --start overrides, 4. --no-resume starts at 0
launch "$T/state" "--size 1 --wpm 50 --start 5 '$book'"
w="$(waitword 'w0[0-9][0-9][0-9]')" || exit 1; test "$w" = w0005 || fail "--start 5 showed $w"
quit || exit 1
launch "$T/state" "--size 1 --wpm 50 --no-resume '$book'"
w="$(waitword 'w0[0-9][0-9][0-9]')" || exit 1; test "$w" = w0000 || fail "--no-resume showed $w"
quit || exit 1; test "$(cat "$T/rc")" = 0 || fail "q exit"
# negative: corrupt entry -> one warning, start at 0, exit 0, entry repaired
printf '{' > "$e"
launch "$T/state" "--size 1 --wpm 50 '$book'"
w="$(waitword 'w0[0-9][0-9][0-9]')" || exit 1; test "$w" = w0000 || fail "corrupt entry showed $w"
quit || exit 1; test "$(cat "$T/rc")" = 0 || fail "corrupt entry exit $(cat "$T/rc")"
test "$(grep -c 'warning' "$T/err")" -eq 1 || fail "want one warning"
test -n "$(idx "$e")" || fail "entry not repaired"
# 5. reaching the end deletes the entry
launch "$T/state" "--size 1 --wpm 1500 --start 196 '$book'"
waitrc || exit 1; test "$(cat "$T/rc")" = 0 || fail "end exit"
test ! -e "$e" && test -z "$(ls -A "$st")" || fail "entry not deleted at end"
# negative: unwritable state dir -> exit 1, one line
printf x > "$T/blocker"
launch "$T/blocker/x" "--size 1 --wpm 50 --no-resume '$book'"
waitword 'w0000' >/dev/null || exit 1; quit || exit 1
test "$(cat "$T/rc")" = 1 && test "$(wc -l < "$T/err")" -eq 1 && grep -q '^fastread: ' "$T/err" || fail "unwritable state dir"
# negative: raw text and stdin never create state
launch "$T/state2" "--size 1 --wpm 1500 'a b c'"; waitrc || exit 1; test "$(cat "$T/rc")" = 0 || fail "raw end"
launch "$T/state2" "--size 1 --wpm 50 'xq1 yq2 zq3'"; waitword 'xq1' >/dev/null || exit 1; quit || exit 1
launch "$T/state2" "--size 1 --wpm 1500" "printf 'a b c' |"; waitrc || exit 1; test "$(cat "$T/rc")" = 0 || fail "stdin end"
test ! -e "$T/state2" || fail "raw/stdin created state"
test "$pre" -eq 1 || test ! -e "$HOME/.local/state/fastread" || fail "real state dir created"
CMD
finish
