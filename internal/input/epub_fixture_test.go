package input

import (
	"archive/zip"
	"bytes"
	"testing"
)

type epubEntry struct {
	Name string
	Data []byte
}

// epubZip writes the entries in order with archive/zip (Deflate; "mimetype" Stored), fixed
// zero modification time, so the output is deterministic.
func epubZip(t *testing.T, entries []epubEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		method := zip.Deflate
		if e.Name == "mimetype" {
			method = zip.Store
		}
		f, err := w.CreateHeader(&zip.FileHeader{Name: e.Name, Method: method})
		if err != nil {
			t.Fatalf("zip %q: %v", e.Name, err)
		}
		if _, err := f.Write(e.Data); err != nil {
			t.Fatalf("zip %q: %v", e.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// epubContainer returns a META-INF/container.xml entry pointing at opfPath.
func epubContainer(opfPath string) epubEntry {
	return epubEntry{"META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="` + opfPath + `" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`)}
}

// epubOPF returns a package document with the given manifest and spine bodies.
func epubOPF(manifest, spine string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Sample</dc:title></metadata>
  <manifest>` + manifest + `</manifest>
  <spine>` + spine + `</spine>
</package>`)
}

// epubXHTML wraps body in a minimal XHTML document.
func epubXHTML(body string) []byte {
	return []byte(`<?xml version="1.0" encoding="utf-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body>` + body + `</body></html>`)
}

// sampleEPUBEntries returns the entries of the committed testdata/sample.epub.
func sampleEPUBEntries() []epubEntry {
	return []epubEntry{
		{"mimetype", []byte("application/epub+zip")},
		epubContainer("OEBPS/content.opf"),
		{"OEBPS/content.opf", epubOPF(`
    <item id="x3" href="text/c.xhtml" media-type="application/xhtml+xml"/>
    <item id="x1" href="text/a.xhtml" media-type="application/xhtml+xml"/>
    <item id="x2" href="text/b.xhtml" media-type="application/xhtml+xml"/>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="style.css" media-type="text/css"/>
  `, `
    <itemref idref="x2"/>
    <itemref idref="x3"/>
    <itemref idref="x1"/>
    <itemref idref="nav" linear="no"/>
  `)},
		{"OEBPS/text/c.xhtml", epubXHTML(`<p>Two second</p>`)},
		{"OEBPS/text/a.xhtml", epubXHTML(`<p>Three <em>th</em>ird</p>`)},
		{"OEBPS/text/b.xhtml", []byte(`<?xml version="1.0" encoding="utf-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Head Title</title><style>p{color:red}</style></head><body><h1>One</h1><p>first &amp; caf&eacute;</p><script>var hidden = 1;</script></body></html>`)},
		{"OEBPS/nav.xhtml", epubXHTML(`<p>Skipped navigation</p>`)},
		{"OEBPS/style.css", []byte("p { margin: 0; }\n")},
	}
}
