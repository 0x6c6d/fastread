#!/usr/bin/env bash
# Verification for T022 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "check command" 900 <<'CMD'
check
CMD
for t in TestPDFCorrupt TestPDFText TestPDFNoText TestPDFBasicCorrupt; do
expect_ok "$t passes" 180 <<CMD
passes $t internal/input
CMD
done
expect_ok "TestPDFCorrupt passes under -race" 300 <<'CMD'
go test -race -count=1 -timeout 240s -run '^TestPDFCorrupt$' ./internal/input
CMD
expect_ok "test file declares the hostile rows" <<'CMD'
f=internal/input/pdf_corrupt_test.go
grep -qF 'func TestPDFCorrupt(' "$f" && grep -qF 'func rawPDF(objects []string) []byte' "$f" &&
grep -qF '2000000000' "$f" && grep -q 'NumGoroutine' "$f" && grep -q 'Encrypt' "$f"
CMD
expect_ok "negative: hostile PDFs exit 1 within 5 s, one line, no panic trace" 120 <<'CMD'
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
go build -o "$T/fr" ./cmd/fastread || exit 1
s=internal/input/testdata/sample.pdf
head -c $(( $(wc -c < "$s") / 2 )) "$s" > "$T/half.pdf"
cat > "$T/mk.py" <<'PY'
import sys
def pdf(objs):
    out = bytearray(b"%PDF-1.4\n"); offs = []
    for i, o in enumerate(objs, 1):
        offs.append(len(out)); out += b"%d 0 obj\n%s\nendobj\n" % (i, o.encode())
    x = len(out)
    out += b"xref\n0 %d\n0000000000 65535 f \n" % (len(objs) + 1)
    for o in offs: out += b"%010d 00000 n \n" % o
    out += b"trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n" % (len(objs) + 1, x)
    return bytes(out)
font = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
def stream(s): return "<< /Length %d >>\nstream\n%s\nendstream" % (len(s), s)
cases = {
  "selfkids.pdf": ["<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [2 0 R] /Count 1 >>"],
  "notj.pdf": ["<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
               "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
               font, stream("BT /F1 12 Tf 72 720 Td Tj ET")],
  "count.pdf": ["<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [] /Count 2000000000 >>"],
}
for name, objs in cases.items():
    open(sys.argv[1] + "/" + name, "wb").write(pdf(objs))
PY
python3 -I "$T/mk.py" "$T" || exit 1
for f in half selfkids notj count; do
  s0=$(date +%s%N)
  timeout -k 2 5 env XDG_STATE_HOME="$T/state" "$T/fr" --no-resume "$T/$f.pdf" </dev/null >"$T/out" 2>"$T/err"; rc=$?
  ms=$(( ($(date +%s%N) - s0) / 1000000 ))
  echo "$f: rc=$rc ${ms}ms: $(head -1 "$T/err")"
  test "$rc" -eq 1 && test "$(wc -l < "$T/err")" -eq 1 && grep -q '^fastread: ' "$T/err" &&
    ! grep -qi -e 'panic' -e 'goroutine' "$T/err" || exit 1
done
CMD
finish
