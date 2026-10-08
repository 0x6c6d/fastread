package input

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const drmMessage = "encrypted EPUB (DRM) not supported"

// encryptionXML returns a META-INF/encryption.xml entry with the given body inside <encryption>.
func encryptionXML(body string) epubEntry {
	return epubEntry{"META-INF/encryption.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" xmlns:xenc="http://www.w3.org/2001/04/xmlenc#">` +
		body + `</encryption>`)}
}

// encryptedData returns one xenc:EncryptedData element for uri with the given algorithm.
func encryptedData(algorithm, uri string) string {
	return `<xenc:EncryptedData><xenc:EncryptionMethod Algorithm="` + algorithm + `"/>` +
		`<xenc:CipherData><xenc:CipherReference URI="` + uri + `"/></xenc:CipherData></xenc:EncryptedData>`
}

const aes128 = "http://www.w3.org/2001/04/xmlenc#aes128-cbc"

// sampleWith returns sampleEPUBEntries plus extra.
func sampleWith(extra ...epubEntry) []epubEntry {
	return append(sampleEPUBEntries(), extra...)
}

// sampleSpaced is the sample with OEBPS/text/a.xhtml stored as "OEBPS/text/a b.xhtml".
func sampleSpaced(extra ...epubEntry) []epubEntry {
	var out []epubEntry
	for _, e := range sampleEPUBEntries() {
		switch e.Name {
		case "OEBPS/content.opf":
			e.Data = bytes.Replace(e.Data, []byte(`href="text/a.xhtml"`), []byte(`href="text/a%20b.xhtml"`), 1)
		case "OEBPS/text/a.xhtml":
			e.Name = "OEBPS/text/a b.xhtml"
		}
		out = append(out, e)
	}
	return append(out, extra...)
}

// rawGarbageZip writes entries like epubZip, but the entry named garbage is written with
// CreateRaw as a Deflate entry whose data is not valid deflate.
func rawGarbageZip(t *testing.T, entries []epubEntry, garbage string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		if e.Name == garbage {
			junk := []byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa, 0xf9, 0xf8}
			f, err := w.CreateRaw(&zip.FileHeader{Name: e.Name, Method: zip.Deflate,
				CRC32: 0x12345678, CompressedSize64: uint64(len(junk)), UncompressedSize64: 100})
			if err != nil {
				t.Fatalf("zip raw %q: %v", e.Name, err)
			}
			if _, err := f.Write(junk); err != nil {
				t.Fatalf("zip raw %q: %v", e.Name, err)
			}
			continue
		}
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

func TestEPUBEncrypted(t *testing.T) {
	sampleWords := []string{"One", "first & café", "Two second", "Three third"}
	font := epubEntry{"OEBPS/fonts/f.otf", []byte("\x00font")}
	deep := strings.Repeat("<x>", 300) + strings.Repeat("</x>", 300)
	lim := Limits{MaxBytes: 1 << 20, MaxEntries: 50, MaxDepth: 256}

	tests := []struct {
		name string
		b    []byte
		want string // "drm", "load", "corrupt" or "deep"
	}{
		{"spine item encrypted", epubZip(t, sampleWith(encryptionXML(encryptedData(aes128, "OEBPS/text/b.xhtml")))), "drm"},
		{"percent-encoded URI", epubZip(t, sampleSpaced(encryptionXML(encryptedData(aes128, "OEBPS/text/a%20b.xhtml")))), "drm"},
		{"leading slash URI", epubZip(t, sampleWith(encryptionXML(encryptedData(aes128, "/OEBPS/text/c.xhtml")))), "drm"},
		{"dot-slash URI", epubZip(t, sampleWith(encryptionXML(encryptedData(aes128, "./OEBPS/text/a.xhtml")))), "drm"},
		{"encrypted spine item never read", rawGarbageZip(t, sampleWith(encryptionXML(encryptedData(aes128, "OEBPS/text/b.xhtml"))), "OEBPS/text/b.xhtml"), "drm"},
		{"obfuscated font only", epubZip(t, sampleWith(font, encryptionXML(encryptedData("http://www.idpf.org/2008/embedding", "OEBPS/fonts/f.otf")))), "load"},
		{"no CipherReference", epubZip(t, sampleWith(encryptionXML(`<xenc:EncryptedKey/>`))), "load"},
		{"rights.xml only", epubZip(t, sampleWith(epubEntry{"META-INF/rights.xml", []byte(`<?xml version="1.0"?><rights xmlns="http://ns.adobe.com/adept"><licenseToken/></rights>`)})), "load"},
		{"encryption.xml cut off inside a tag", epubZip(t, sampleWith(epubEntry{"META-INF/encryption.xml",
			[]byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><xenc:EncryptedData xmlns:xenc="http://www.w3.org/2001/04/xmlenc#"><xenc:CipherRef`)})), "corrupt"},
		{"encryption.xml 300 deep, MaxDepth 256", epubZip(t, sampleWith(encryptionXML(deep))), "deep"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := loadEPUB(tc.b, lim)
			switch tc.want {
			case "drm":
				if !errors.Is(err, ErrUnsupported) || errors.Is(err, ErrCorruptEPUB) {
					t.Fatalf("err = %v, want ErrUnsupported and not ErrCorruptEPUB", err)
				}
				if !strings.Contains(err.Error(), drmMessage) {
					t.Fatalf("err = %q, want it to contain %q", err, drmMessage)
				}
			case "load":
				if err != nil {
					t.Fatalf("loadEPUB: %v", err)
				}
				if !reflect.DeepEqual(got, sampleWords) {
					t.Fatalf("got %q, want %q", got, sampleWords)
				}
			case "corrupt":
				if !errors.Is(err, ErrCorruptEPUB) {
					t.Fatalf("err = %v, want ErrCorruptEPUB", err)
				}
			case "deep":
				if !errors.Is(err, ErrCorruptEPUB) || !errors.Is(err, ErrTooLarge) {
					t.Fatalf("err = %v, want ErrCorruptEPUB and ErrTooLarge", err)
				}
			}
		})
	}

	t.Run("LoadFile on drm.epub", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "drm.epub")
		if err := os.WriteFile(p, tests[0].b, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadFile(p)
		if !errors.Is(err, ErrUnsupported) || errors.Is(err, ErrCorruptEPUB) {
			t.Fatalf("err = %v, want ErrUnsupported and not ErrCorruptEPUB", err)
		}
	})
}
