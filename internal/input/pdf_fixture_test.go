package input

import (
	"bytes"
	"fmt"
	"strings"
)

// buildPDF returns a minimal, deterministic PDF 1.4: catalog, one Pages node, one page per
// string (MediaBox 0 0 612 792, Helvetica Type1 font /F1), content stream
// "BT /F1 12 Tf 72 720 Td (<text>) Tj ET" with ( ) \ escaped, or an empty stream (Length 0)
// for "", a correct xref table (10-digit offsets), trailer /Size /Root, startxref, %%EOF.
func buildPDF(pages []string) []byte {
	// Object numbers: 1 catalog, 2 pages, 3 font, then per page i: 4+2i page, 5+2i content.
	n := len(pages)
	objs := make([]string, 0, 3+2*n)
	objs = append(objs, "<< /Type /Catalog /Pages 2 0 R >>")
	kids := make([]string, n)
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	objs = append(objs, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n))
	objs = append(objs, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	esc := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	for i, text := range pages {
		objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "+
			"/Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+2*i))
		content := ""
		if text != "" {
			content = "BT /F1 12 Tf 72 720 Td (" + esc.Replace(text) + ") Tj ET"
		}
		objs = append(objs, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return buf.Bytes()
}
