#!/usr/bin/env bash
# Verification for T070 (supervisor-owned). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
expect_ok "T068 verification still passes" 900 <<'CMD'
bash scripts/verify/T068.sh >/dev/null
CMD
expect_ok "help text says --no-resume ignores the saved position, not that it does not save" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread && "$T/fr" --help >"$T/h" &&
grep -qF -- '--no-resume      ignore the saved resume position at start' "$T/h" && ! grep -q 'or save' "$T/h" &&
! grep -q 'even though' docs/usage.md && ! grep -q 'As built' docs/usage.md
CMD
expect_ok "protected files unchanged" <<'CMD'
scripts/check-protected.sh
CMD
finish
