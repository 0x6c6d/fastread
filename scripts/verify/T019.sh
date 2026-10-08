#!/usr/bin/env bash
# Verification for T019 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestEPUBEncrypted TestEPUBSpineOrder TestEPUBCorrupt TestEPUBLimits; do
expect_ok "$t passes" <<CMD
passes $t internal/input
CMD
done
expect_ok "TestEPUB* pass under -race" <<'CMD'
go test -race -count=1 -run '^TestEPUB' ./internal/input
CMD
expect_ok "checkEncryption declared and called in loadEPUB before the spine reads" <<'CMD'
grep -qF 'func checkEncryption(z *zipArchive, spine []string, lim Limits) error' internal/input/epub_drm.go &&
grep -qF 'encrypted EPUB (DRM) not supported' internal/input/epub_drm.go &&
body="$(awk '/^func loadEPUB\(/,/^}/' internal/input/epub.go)" &&
c=$(printf '%s\n' "$body" | grep -n 'checkEncryption(' | head -1 | cut -d: -f1) &&
r=$(printf '%s\n' "$body" | grep -n '\.read(' | head -1 | cut -d: -f1) &&
test -n "$c" && test -n "$r" && test "$c" -lt "$r"
CMD
expect_ok "negative: DRM EPUB exits 1 with one line mentioning DRM; font-only encryption loads" <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
cat > "$T/mk.py" <<'PY'
import sys, zipfile
src, dst, uri = sys.argv[1], sys.argv[2], sys.argv[3]
enc = ('<?xml version="1.0" encoding="UTF-8"?>'
       '<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" '
       'xmlns:enc="http://www.w3.org/2001/04/xmlenc#"><enc:EncryptedData>'
       '<enc:EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/>'
       '<enc:CipherData><enc:CipherReference URI="%s"/></enc:CipherData>'
       '</enc:EncryptedData></encryption>') % uri
with zipfile.ZipFile(src) as zi, zipfile.ZipFile(dst, "w", zipfile.ZIP_DEFLATED) as zo:
    for info in zi.infolist():
        zo.writestr(info, zi.read(info.filename))
    zo.writestr("META-INF/encryption.xml", enc)
    zo.writestr("OEBPS/fonts/f.otf", b"\x00font")
PY
python3 -I "$T/mk.py" internal/input/testdata/sample.epub "$T/drm.epub" OEBPS/text/b.xhtml || exit 1
python3 -I "$T/mk.py" internal/input/testdata/sample.epub "$T/font.epub" OEBPS/fonts/f.otf || exit 1
env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/drm.epub" </dev/null >"$T/out" 2>"$T/err"; rc=$?
cat "$T/err"
test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && grep -q 'DRM' "$T/err" &&
! grep -q 'corrupt' "$T/err" || exit 1
setsid -w env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/font.epub" </dev/null >"$T/out" 2>"$T/err2"; rc=$?
cat "$T/err2"
test "$rc" -eq 1 && ! grep -qi -e 'DRM' -e 'corrupt' -e 'unsupported' "$T/err2"
CMD
expect_fail "negative: crypto package imported in internal/input" <<'CMD'
grep -rn --include='*.go' '"crypto/' internal/input | grep -v '"crypto/sha256"'
CMD
finish
