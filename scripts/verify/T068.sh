#!/usr/bin/env bash
# Verification for T068 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Every section check reads the section with: awk -v h='## X' '$0==h{f=1;next} /^## /{f=0} f'
. "$(dirname "$0")/_lib.sh"
expect_ok "check passes" 1800 <<'CMD'
check
CMD
expect_ok "Plan D14 (deliverables) verbatim" <<'CMD'
head -1 LICENSE | grep -qx 'MIT License' && grep -q 'Lucas Menke' LICENSE && test -s third_party/gofont/LICENSE && test -s third_party/misc-fixed/LICENSE && for h in '## Install and build' '## Examples' '## Flags' '## Keys' '## Dependencies' '## GUI font' '## Limitations'; do grep -qF "$h" docs/usage.md || exit 1; done && for x in stdin .txt .md .epub .fb2 .pdf; do grep -qF -- "$x" docs/usage.md || exit 1; done
CMD
expect_ok "title, RSVP intro, the seven sections as headings in order; only docs/usage.md under docs/" <<'CMD'
head -1 docs/usage.md | grep -qx '# fastread usage' || { echo "first line is not '# fastread usage'"; exit 1; }
awk '/^## /{exit} {print}' docs/usage.md | grep -q 'RSVP' || { echo "intro lacks RSVP"; exit 1; }
prev=0
for h in '## Install and build' '## Examples' '## Flags' '## Keys' '## Dependencies' '## GUI font' '## Limitations'; do
  n="$(grep -nxF -- "$h" docs/usage.md | head -1 | cut -d: -f1)"
  test -n "$n" && test "$n" -gt "$prev" || { echo "heading '$h' missing or out of order"; exit 1; }
  prev="$n"
done
test "$(find docs -type f | sort | tr '\n' ' ')" = "docs/usage.md " || { find docs -type f; exit 1; }
CMD
expect_ok "Install and build: cgo/Gio libs, nogui, CGO_ENABLED=0, Go 1.24, GOTOOLCHAIN, tests" <<'CMD'
s="$(awk -v h='## Install and build' '$0==h{f=1;next} /^## /{f=0} f' docs/usage.md)"
for p in 'go build' '-tags nogui' 'CGO_ENABLED=0' 'libx11-xcb-dev' '1.24' 'GOTOOLCHAIN=local' '-tags e2e' 'go test'; do
  printf '%s\n' "$s" | grep -qF -- "$p" || { echo "Install and build lacks: $p"; exit 1; }
done
CMD
expect_ok "Examples: raw, stdin, txt, md, epub, fb2, pdf, --ui gui, '--' line; path-like paragraph" <<'CMD'
s="$(awk -v h='## Examples' '$0==h{f=1;next} /^## /{f=0} f' docs/usage.md)"
printf '%s\n' "$s" | grep -qE "fastread [^|]*(\"[^\"]* [^\"]*\"|'[^']* [^']*')" || { echo "no quoted raw-text example"; exit 1; }
printf '%s\n' "$s" | grep -qE '\| *fastread' || { echo "no stdin (pipe) example"; exit 1; }
for x in txt md epub fb2 pdf; do
  printf '%s\n' "$s" | grep -qE "fastread [^|]*\.$x([^a-zA-Z0-9]|\$)" || { echo "no .$x example"; exit 1; }
done
printf '%s\n' "$s" | grep -qF -- '--ui gui' || { echo "no --ui gui example"; exit 1; }
printf '%s\n' "$s" | grep -qF -- 'fastread -- --wpm' || { echo "no 'fastread -- --wpm' line"; exit 1; }
printf '%s\n' "$s" | grep -qxF '### Text or file?' || { echo "no '### Text or file?' subsection"; exit 1; }
printf '%s\n' "$s" | awk -v RS= '/path-like/ && /whitespace/ && /~/ && /\.pdf/ && /not found/ {ok=1} END {exit !ok}' || { echo "no paragraph documenting the path-like rule (path-like, whitespace, ~, .pdf, not found)"; exit 1; }
CMD
expect_ok "Flags: every --help flag, defaults and ranges, exit codes, resume, environment" 300 <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
"$T/fr" --help >"$T/help" || exit 1
s="$(awk -v h='## Flags' '$0==h{f=1;next} /^## /{f=0} f' docs/usage.md)"
for fl in $(grep -oE -- '--[a-z][a-z-]*' "$T/help" | sort -u) -h; do
  printf '%s\n' "$s" | grep -qF -- "$fl" || { echo "Flags lacks $fl"; exit 1; }
done
test "$(printf '%s\n' "$s" | grep -c '^|')" -ge 8 || { echo "Flags table has fewer than 8 rows"; exit 1; }
for p in 300 50 1500 XDG_STATE_HOME .local/state/fastread 0700 0600 SHA-256 NO_COLOR COLORTERM WAYLAND_DISPLAY; do
  printf '%s\n' "$s" | grep -qF -- "$p" || { echo "Flags lacks: $p"; exit 1; }
done
printf '%s\n' "$s" | grep -qi 'exit code' || { echo "Flags lacks exit codes"; exit 1; }
CMD
expect_ok "Keys: every live key and the HelpText from internal/tui verbatim" <<'CMD'
s="$(awk -v h='## Keys' '$0==h{f=1;next} /^## /{f=0} f' docs/usage.md)"
for k in Space Up Down Left Right Home Esc Ctrl+C '`p`' '`?`' '`q`' '`[`' '`]`' 25 10; do
  printf '%s\n' "$s" | grep -qF -- "$k" || { echo "Keys lacks: $k"; exit 1; }
done
h="$(grep -rhoE 'HelpText *= *"[^"]*"' internal/tui/*.go | head -1 | sed 's/^[^"]*"//; s/"$//')"
test -n "$h" || { echo "cannot read HelpText from internal/tui"; exit 1; }
printf '%s\n' "$s" | grep -qF -- "$h" || { echo "Keys lacks HelpText verbatim: $h"; exit 1; }
CMD
expect_ok "Dependencies: every go.mod require module (direct and indirect), Xvfb and tmux" <<'CMD'
s="$(awk -v h='## Dependencies' '$0==h{f=1;next} /^## /{f=0} f' docs/usage.md)"
mods="$(awk '/^require \(/{f=1;next} f&&/^\)/{f=0;next} f&&NF{print $1} /^require [^(]/{print $2}' go.mod)"
test -n "$mods" || { echo "no require lines in go.mod"; exit 1; }
for m in $mods; do printf '%s\n' "$s" | grep -qF -- "$m" || { echo "Dependencies lacks $m"; exit 1; }; done
for p in Xvfb tmux; do printf '%s\n' "$s" | grep -qF -- "$p" || { echo "Dependencies lacks $p"; exit 1; }; done
CMD
expect_ok "GUI font: goregular, licence paths, LevelSp and MinSp from internal/gui, width/2, misc-fixed" <<'CMD'
s="$(awk -v h='## GUI font' '$0==h{f=1;next} /^## /{f=0} f' docs/usage.md)"
for p in goregular third_party/gofont/LICENSE misc-fixed third_party/misc-fixed/LICENSE width/2; do
  printf '%s\n' "$s" | grep -qF -- "$p" || { echo "GUI font lacks: $p"; exit 1; }
done
lv="$(grep -rhoE 'LevelSp *= *\[[0-9]*\]int *\{[^}]*\}' internal/gui/*.go | head -1 | sed 's/.*{//; s/}//' | tr ',' '\n' | tr -d ' \t' | grep -v '^0$')"
test "$(printf '%s\n' "$lv" | grep -c .)" -eq 5 || { echo "cannot read 5 LevelSp values: $lv"; exit 1; }
for v in $lv; do printf '%s\n' "$s" | grep -qw -- "$v" || { echo "GUI font lacks LevelSp value $v"; exit 1; }; done
m="$(grep -rhoE '\bMinSp *= *[0-9]+' internal/gui/*.go | head -1 | grep -oE '[0-9]+$')"
test -n "$m" || { echo "cannot read MinSp"; exit 1; }
printf '%s\n' "$s" | grep -qE -- "(^|[^0-9])$m ?sp\b" || { echo "GUI font lacks the shrink minimum '$m sp'"; exit 1; }
CMD
expect_ok "Limitations: platforms, scanned PDF, DRM, RTL, Wayland, size cap, fb2.zip" <<'CMD'
s="$(awk -v h='## Limitations' '$0==h{f=1;next} /^## /{f=0} f' docs/usage.md)"
for p in Windows scanned DRM Wayland '256 MiB' '.fb2.zip'; do
  printf '%s\n' "$s" | grep -qF -- "$p" || { echo "Limitations lacks: $p"; exit 1; }
done
printf '%s\n' "$s" | grep -q -e 'RTL' -e 'right-to-left' || { echo "Limitations lacks RTL"; exit 1; }
CMD
expect_ok "README.md and CLAUDE.md unchanged; protected files unchanged" <<'CMD'
git diff --quiet HEAD -- README.md CLAUDE.md && scripts/check-protected.sh
CMD
expect_fail "negative: docs name a --flag the binary does not accept (fastread lines, Flags section) or mention font8x8" 300 <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
test -f docs/usage.md || exit 0
go build -o "$T/fr" ./cmd/fastread || exit 0
"$T/fr" --help | grep -oE -- '--[a-z][a-z-]*' | sort -u >"$T/ok"
{ grep -F 'fastread ' docs/usage.md; awk -v h='## Flags' '$0==h{f=1;next} /^## /{f=0} f' docs/usage.md; } |
  grep -oE -- '(^|[^a-zA-Z0-9-])--[a-z][a-z-]*' | grep -oE -- '--[a-z][a-z-]*' | sort -u >"$T/doc"
bad="$(comm -23 "$T/doc" "$T/ok")"
test -n "$bad" && { echo "unknown flags in docs: $bad"; exit 0; }
grep -n -i 'font8x8' docs/usage.md && exit 0
exit 1
CMD
finish
