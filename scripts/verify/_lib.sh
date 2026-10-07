# Shared helpers for scripts/verify/<ID>.sh (supervisor-owned; workers never edit it).
# Source it, then call expect_ok / expect_fail per check and finish at the end.
#
#   expect_ok   <label> [timeout_s]  reads ONE command from stdin (quoted heredoc), runs it with
#                                    bash -c from the repo root; passes iff it exits 0.
#   expect_fail <label> [timeout_s]  same, but passes iff it exits with an ordinary failure code
#                                    (1-123). Exit 0, a timeout (124/137), a timeout error (125),
#                                    "not executable" (126), "command not found" (127) or death by
#                                    signal (>=128) count as FAILURE of the check.
#   finish                           prints the summary; exits 1 if any check failed.
#
# Default timeout per check: $VERIFY_TIMEOUT seconds (default 300).
# Exported to every command: GOTOOLCHAIN=local, and the functions `check` (the Plan.md check
# command) and `passes <TestName> <pkgdir>` (the test exists, ran and printed --- PASS).

_VERIFY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_VERIFY_TOTAL=0
_VERIFY_FAILED=0
export GOTOOLCHAIN=local

check() {
  test -z "$(gofmt -l .)" &&
    go vet ./... && go vet -tags nogui ./... && go vet -tags e2e ./... &&
    go test ./... && go test -race ./... && go test -tags nogui ./... &&
    go build ./... && go build -tags nogui ./... && CGO_ENABLED=0 go build -tags nogui ./...
}
export -f check

passes() {
  local out rc
  out="$(go test -count=1 -run "^$1\$" -v "./$2" 2>&1)"
  rc=$?
  if [ "$rc" -ne 0 ]; then
    printf '%s\n' "$out" | tail -n 30 >&2
    return 1
  fi
  printf '%s\n' "$out" | grep -qE -- "^--- PASS: $1 \(" || {
    echo "passes: no '--- PASS: $1' line in ./$2" >&2
    return 1
  }
}
export -f passes

_verify_run() {
  local mode="$1" label="$2" secs="${3:-${VERIFY_TIMEOUT:-300}}" cmd out rc ok=0
  cmd="$(cat)"
  _VERIFY_TOTAL=$((_VERIFY_TOTAL + 1))
  out="$(cd "$_VERIFY_ROOT" && timeout -k 10 "$secs" bash -c "$cmd" 2>&1)"
  rc=$?
  if [ "$mode" = ok ]; then
    [ "$rc" -eq 0 ] && ok=1
  else
    [ "$rc" -ge 1 ] && [ "$rc" -le 123 ] && ok=1
  fi
  if [ "$ok" -eq 1 ]; then
    printf 'ok    %s\n' "$label"
  else
    _VERIFY_FAILED=$((_VERIFY_FAILED + 1))
    if [ "$rc" -eq 124 ] || [ "$rc" -eq 137 ]; then
      printf 'FAIL  %s (timeout after %ss)\n' "$label" "$secs"
    else
      printf 'FAIL  %s (exit %s, expected %s)\n' "$label" "$rc" \
        "$([ "$mode" = ok ] && echo 0 || echo '1-123')"
    fi
    printf '%s\n' "$out" | tail -n 25 | sed 's/^/      | /'
  fi
}

expect_ok() { _verify_run ok "$@"; }
expect_fail() { _verify_run fail "$@"; }

finish() {
  if [ "$_VERIFY_FAILED" -eq 0 ]; then
    printf 'PASS  %d/%d checks\n' "$_VERIFY_TOTAL" "$_VERIFY_TOTAL"
    exit 0
  fi
  printf 'FAIL  %d of %d checks failed\n' "$_VERIFY_FAILED" "$_VERIFY_TOTAL"
  exit 1
}
