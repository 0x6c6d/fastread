#!/usr/bin/env bash
# Verification for T016 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestMarkdownStrip TestLoadFile; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "signature and committed fixture" <<'CMD'
grep -qF 'func loadMarkdown(b []byte) ([]string, error)' internal/input/markdown.go &&
f=internal/input/testdata/sample.md && test -f "$f" &&
head -1 "$f" | grep -qx '# Title Here' &&
grep -qF 'Some *emphasis*, **strong** and _under_ text with snake_case kept.' "$f" &&
grep -qF '![alt words](img.png)' "$f" && grep -qF 'fenced code dropped' "$f" &&
grep -qF 'Escaped 5 \* 3 and <span>html</span> plus [ref link][ref] too.' "$f" &&
tail -1 "$f" | grep -qx '\[ref\]: http://example.com/ref'
CMD
expect_ok "negative: fenced-code-only .md file -> exit 1 'empty text'" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
printf '```sh\nrm -rf /\n```\n' > "$T/code.md"
env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/code.md" </dev/null 2>"$T/err"; rc=$?
cat "$T/err"
test "$rc" -eq 1 && grep -q 'empty text' "$T/err"
CMD
finish
