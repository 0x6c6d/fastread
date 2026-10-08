package input

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// rawPDF writes a PDF 1.4 whose objects are numbered from 1 in order, with a correct xref
// table and a trailer "<< /Size n /Root 1 0 R >>".
func rawPDF(objects []string) []byte {
	return rawPDFTrailer(objects, "")
}

// rawPDFTrailer is rawPDF with extra trailer entries (e.g. "/Encrypt 5 0 R").
func rawPDFTrailer(objects []string, extra string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	if extra != "" {
		extra = " " + extra
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R%s >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, extra, xref)
	return buf.Bytes()
}

// pdfStream returns a stream object with a correct /Length.
func pdfStream(s string) string {
	return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(s), s)
}

const (
	pdfCatalog = "<< /Type /Catalog /Pages 2 0 R >>"
	pdfFont    = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
)

// pdfPage returns a Page object with parent `parent`, font object `font` and content `content`
// (content <= 0: no /Contents key).
func pdfPage(parent, font, content int) string {
	c := ""
	if content > 0 {
		c = fmt.Sprintf(" /Contents %d 0 R", content)
	}
	return fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 612 792] "+
		"/Resources << /Font << /F1 %d 0 R >> >>%s >>", parent, font, c)
}

// onePagePDF: catalog, Pages, one page (obj 3), font (obj 4), content stream (obj 5).
func onePagePDF(content string) []byte {
	return rawPDF([]string{pdfCatalog, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		pdfPage(2, 4, 5), pdfFont, pdfStream(content)})
}

// nestedPagesPDF: Pages nested depth levels deep (objects 2..depth+1), the innermost holding
// one page with text.
func nestedPagesPDF(depth int) []byte {
	objs := []string{pdfCatalog}
	for i := 0; i < depth; i++ {
		n := 2 + i
		parent := ""
		if i > 0 {
			parent = fmt.Sprintf(" /Parent %d 0 R", n-1)
		}
		objs = append(objs, fmt.Sprintf("<< /Type /Pages%s /Kids [%d 0 R] /Count 1 >>", parent, n+1))
	}
	page := len(objs) + 1
	objs = append(objs, pdfPage(page-1, page+1, page+2), pdfFont,
		pdfStream("BT /F1 12 Tf 72 720 Td (deep) Tj ET"))
	return rawPDF(objs)
}

// xrefAllZero rewrites every in-use xref entry of b to offset 0.
func xrefAllZero(b []byte) []byte {
	i := bytes.Index(b, []byte("xref\n"))
	head, tail := b[:i], string(b[i:])
	lines := strings.Split(tail, "\n")
	for j, l := range lines {
		if strings.HasSuffix(l, " 00000 n ") {
			lines[j] = "0000000000 00000 n "
		}
	}
	return append(append([]byte{}, head...), strings.Join(lines, "\n")...)
}

// startxrefPastEnd replaces the startxref value with an offset beyond the end of b.
func startxrefPastEnd(b []byte) []byte {
	i := bytes.LastIndex(b, []byte("startxref\n"))
	return append(append([]byte{}, b[:i]...), fmt.Sprintf("startxref\n%d\n%%%%EOF\n", len(b)+1000)...)
}

func TestPDFCorrupt(t *testing.T) {
	sample := samplePDF()
	random := make([]byte, 1024)
	rand.New(rand.NewSource(42)).Read(random)

	encrypted := rawPDFTrailer([]string{pdfCatalog, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		pdfPage(2, 4, 5), pdfFont, pdfStream("BT /F1 12 Tf 72 720 Td (secret) Tj ET"),
		"<< /Filter /Standard /V 1 /R 2 /O (" + strings.Repeat("o", 32) + ") /U (" +
			strings.Repeat("u", 32) + ") /P -4 >>"},
		"/Encrypt 6 0 R /ID [<0123456789abcdef0123456789abcdef> <0123456789abcdef0123456789abcdef>]")

	twoLevel := rawPDF([]string{pdfCatalog,
		"<< /Type /Pages /Kids [3 0 R] /Count 3 >>",
		"<< /Type /Pages /Parent 2 0 R /Kids [4 0 R 5 0 R 6 0 R] /Count 3 >>",
		pdfPage(3, 7, 8), pdfPage(3, 7, 9), pdfPage(3, 7, 10), pdfFont,
		pdfStream("BT /F1 12 Tf 72 720 Td (First leaf.) Tj ET"),
		pdfStream("BT /F1 12 Tf 72 720 Td (Second leaf.) Tj ET"),
		pdfStream("BT /F1 12 Tf 72 720 Td (Third leaf.) Tj ET")})

	corrupt := []error{ErrCorruptPDF}
	tests := []struct {
		name string
		b    []byte
		lim  Limits
		// want: the error must match one of these (nil slice: success expected).
		want []error
		// texts: expected words per paragraph on success.
		texts [][]string
	}{
		{name: "header only", b: []byte("%PDF-1.4\n"), want: corrupt},
		{name: "header plus 1 KiB random", b: append([]byte("%PDF-1.4\n"), random...), want: corrupt},
		{name: "truncated 25%", b: sample[:len(sample)/4], want: corrupt},
		{name: "truncated 50%", b: sample[:len(sample)/2], want: corrupt},
		{name: "truncated 90%", b: sample[:len(sample)*9/10], want: corrupt},
		{name: "last 30 bytes removed", b: sample[:len(sample)-30], want: corrupt},
		{name: "startxref past end", b: startxrefPastEnd(sample), want: corrupt},
		{name: "xref offsets all zero", b: xrefAllZero(sample), want: corrupt},
		{name: "Kids contains itself", b: rawPDF([]string{pdfCatalog,
			"<< /Type /Pages /Kids [2 0 R] /Count 1 >>"}), want: corrupt},
		{name: "Pages nested 40 deep", b: nestedPagesPDF(40), want: corrupt},
		{name: "Tj without operand", b: onePagePDF("BT /F1 12 Tf 72 720 Td Tj ET"), want: corrupt},
		{name: "Tf with one operand", b: onePagePDF("BT 12 Tf 72 720 Td (x) Tj ET"), want: corrupt},
		{name: "Root not a dictionary", b: rawPDF([]string{"42"}), want: corrupt},
		{name: "catalog without Pages", b: rawPDF([]string{"<< /Type /Catalog >>"}), want: corrupt},

		{name: "Count 2000000000 with empty Kids", b: rawPDF([]string{pdfCatalog,
			"<< /Type /Pages /Kids [] /Count 2000000000 >>"}), want: []error{ErrNoText}},
		{name: "page without Contents", b: rawPDF([]string{pdfCatalog,
			"<< /Type /Pages /Kids [3 0 R] /Count 1 >>", pdfPage(2, 4, 0), pdfFont}),
			want: []error{ErrNoText}},

		// Observed: ErrNoText (the library resolves the missing object to null, so the page
		// has no content).
		{name: "Contents references missing object", b: rawPDF([]string{pdfCatalog,
			"<< /Type /Pages /Kids [3 0 R] /Count 1 >>", pdfPage(2, 4, 99), pdfFont}),
			want: []error{ErrCorruptPDF, ErrNoText}},

		// Observed: ErrUnsupported (the library reports pdf.ErrInvalidPassword for the empty
		// user password).
		{name: "Encrypt with non-empty user password", b: encrypted,
			want: []error{ErrUnsupported, ErrCorruptPDF}},

		{name: "over size limit", b: sample, lim: Limits{MaxBytes: 100, MaxEntries: 1, MaxDepth: 1},
			want: []error{ErrTooLarge}},

		{name: "two-level Pages tree", b: twoLevel,
			texts: [][]string{{"First", "leaf."}, {"Second", "leaf."}, {"Third", "leaf."}}},
	}

	bound := 2 * time.Second
	if raceEnabled {
		bound = 20 * time.Second
	}
	start := runtime.NumGoroutine()

	type result struct {
		paras []string
		err   error
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lim := tc.lim
			if lim == (Limits{}) {
				lim = DefaultLimits()
			}
			done := make(chan result, 1)
			go func() {
				p, err := loadPDF(tc.b, lim)
				done <- result{p, err}
			}()
			var r result
			select {
			case r = <-done:
			case <-time.After(bound):
				t.Fatalf("loadPDF did not return within %v", bound)
			}
			if tc.want == nil {
				if r.err != nil {
					t.Fatalf("loadPDF: %v", r.err)
				}
				if w := pdfWords(r.paras); !reflect.DeepEqual(w, tc.texts) {
					t.Fatalf("got %q, want %q", w, tc.texts)
				}
				return
			}
			if r.err == nil {
				t.Fatalf("loadPDF succeeded with %q, want one of %v", r.paras, tc.want)
			}
			t.Logf("err = %v", r.err)
			if strings.Contains(r.err.Error(), "\n") {
				t.Errorf("error message spans several lines: %q", r.err)
			}
			matched := 0
			for _, w := range tc.want {
				if errors.Is(r.err, w) {
					matched++
				}
			}
			if matched == 0 {
				t.Fatalf("err = %v, want one of %v", r.err, tc.want)
			}
			// ErrCorruptPDF and ErrNoText are mutually exclusive.
			if errors.Is(r.err, ErrCorruptPDF) && errors.Is(r.err, ErrNoText) {
				t.Fatalf("err = %v matches both ErrCorruptPDF and ErrNoText", r.err)
			}
		})
	}

	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > start && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > start {
		t.Fatalf("NumGoroutine = %d after all rows, started with %d: a row left a goroutine running", n, start)
	}
}
