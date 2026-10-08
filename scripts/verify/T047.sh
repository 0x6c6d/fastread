#!/usr/bin/env bash
# Verification for T047 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "TestKeyDecode passes (incl. negative rows: ignored sequences, Ctrl+C inside a sequence)" <<'CMD'
passes TestKeyDecode internal/tui
CMD
expect_ok "TestKeyDecode passes under -race and -tags nogui" 600 <<'CMD'
go test -race -count=2 -run '^TestKeyDecode$' ./internal/tui &&
go test -tags nogui -count=1 -run '^TestKeyDecode$' ./internal/tui
CMD
expect_ok "FuzzKeyDecode runs 15 s without failure" 300 <<'CMD'
go test -run '^$' -fuzz '^FuzzKeyDecode$' -fuzztime 15s ./internal/tui
CMD
expect_ok "earlier internal/tui tests still pass" 600 <<'CMD'
go test -count=1 ./internal/tui/...
CMD
expect_ok "exported API present" <<'CMD'
test -f internal/tui/keys.go && test -f internal/tui/keys_test.go || exit 1
d="$(go doc -all ./internal/tui)" || exit 1
for s in 'EscTimeout += 50 \* time\.Millisecond' 'type KeyDecoder struct' \
  'func \(d \*KeyDecoder\) Feed\(b \[\]byte\) \[\]state\.Action' \
  'func \(d \*KeyDecoder\) Pending\(\) bool' 'func \(d \*KeyDecoder\) Flush\(\) \[\]state\.Action'; do
  printf '%s\n' "$d" | grep -qE "$s" || { echo "missing: $s"; exit 1; }
done
CMD
expect_fail "negative: keys.go does I/O, sleeps, reads the clock or starts goroutines" <<'CMD'
grep -nE -e '"os"' -e '"io"' -e 'time\.Sleep' -e 'time\.Now' -e 'time\.After' -e '^\s*go ' internal/tui/keys.go
CMD
finish
