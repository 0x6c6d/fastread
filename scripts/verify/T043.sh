#!/usr/bin/env bash
# Verification for T043 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestEncodeStripsControls TestColorModes TestNoColor TestEncodeColors TestDetectColorMode TestProgressHelpFocus TestTicks TestGlyphFallback; do
expect_ok "$t passes" <<CMD
passes $t internal/tui
CMD
done
expect_ok "encode tests pass under -race" 600 <<'CMD'
go test -race -count=1 -run '^(TestEncodeStripsControls|TestColorModes|TestNoColor)$' ./internal/tui
CMD
expect_ok "negative: FuzzRenderEncode finds no leaking input in 15 s" 240 <<'CMD'
go test -count=1 -run '^$' -fuzz '^FuzzRenderEncode$' -fuzztime 15s ./internal/tui
CMD
expect_ok "no fuzz crasher left in the tree" <<'CMD'
test ! -d internal/tui/testdata/fuzz/FuzzRenderEncode || test -z "$(git status --porcelain internal/tui/testdata/fuzz)"
CMD
# Independent hostile-text check (supervisor-owned program, built in a temp package
# inside the module and removed afterwards). Exit 3 = leak found.
read -r -d '' PROG <<'GOEOF'
package main

import (
	"fmt"
	"os"
	"regexp"
	"unicode/utf8"

	"github.com/0x6c6d/fastread/internal/tui"
)

var csi = regexp.MustCompile("\x1b\\[[0-9;]*[Hm]")

func leak(out []byte) string {
	if !utf8.Valid(out) {
		return "invalid UTF-8"
	}
	for _, r := range string(csi.ReplaceAll(out, nil)) {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			return fmt.Sprintf("control rune %U", r)
		}
	}
	return ""
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "selftest" {
		if leak([]byte("ok\x1b]0;x\x07")) == "" {
			os.Exit(0)
		}
		os.Exit(3)
	}
	words := []string{"a\x1b[31mb\x9bc\u202ed\x07e\x00f\xffg\x1b]0;x\x07", "\x1b[2J\x1b[H", "\u009b2J",
		"\x1bP+q\x1b\\", "x\u2066y\u2069z\x7f", "\r\n\t\b", "\xc2\x9b31m", "\xe2\x80\xae"}
	for _, w := range words {
		for _, s := range []int{1, 2, 3, 5} {
			f := tui.Render(tui.Model{Word: w, Size: s, ShowProgress: true, ShowHelp: true, Total: 1}, 80, 24)
			if len(f.Cells) > 1 && len(f.Cells[1]) > 0 {
				f.Cells[1][0].Text = w // a malformed cell straight from untrusted text
			}
			for m := tui.ColorNone; m <= tui.ColorTrue; m++ {
				if why := leak(tui.Encode(f, m)); why != "" {
					fmt.Printf("leak for %q size %d mode %d: %s\n", w, s, m, why)
					os.Exit(3)
				}
			}
		}
	}
}
GOEOF
export PROG
expect_ok "independent hostile-text program finds no control byte in any mode" 300 <<'CMD'
d="$(mktemp -d -p internal/tui zzverify.XXXXXX)" || exit 1; trap 'rm -rf "$d"' EXIT
printf '%s\n' "$PROG" > "$d/main.go" && go run "./$d"
CMD
expect_fail "negative: the hostile-text detector fires on a raw ESC (self-test)" 300 <<'CMD'
d="$(mktemp -d -p internal/tui zzverify.XXXXXX)" || exit 0; trap 'rm -rf "$d"' EXIT
printf '%s\n' "$PROG" > "$d/main.go" || exit 0
go build -o "$d/prog" "./$d" || exit 0
"$d/prog" selftest
CMD
expect_fail "negative: encode.go delegates stripping to internal/tokenize" <<'CMD'
grep -n 'internal/tokenize' internal/tui/encode.go internal/tui/color.go
CMD
expect_ok "cleanCell exists in encode.go" <<'CMD'
grep -qE '^func cleanCell\(s string\) string' internal/tui/encode.go
CMD
expect_ok "no verify temp package left behind" <<'CMD'
test -z "$(ls -d internal/tui/zzverify.* 2>/dev/null)"
CMD
finish
