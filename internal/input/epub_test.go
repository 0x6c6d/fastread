package input

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func xhtmlItem(id, href string) string {
	return `<item id="` + id + `" href="` + href + `" media-type="application/xhtml+xml"/>`
}

// oneItemEPUB is an EPUB whose OPF (at opfPath) lists a single XHTML item with href, stored at
// entry name.
func oneItemEPUB(opfPath, href, name string, body []byte) []epubEntry {
	return []epubEntry{
		{"mimetype", []byte("application/epub+zip")},
		epubContainer(opfPath),
		{opfPath, epubOPF(xhtmlItem("a", href), `<itemref idref="a"/>`)},
		{name, body},
	}
}

func TestEPUBSpineOrder(t *testing.T) {
	sample := epubZip(t, sampleEPUBEntries())
	tests := []struct {
		name    string
		entries []epubEntry
		want    []string
	}{
		{"sample", sampleEPUBEntries(), []string{"One", "first & café", "Two second", "Three third"}},
		{"OPF at the zip root", oneItemEPUB("content.opf", "text/a.xhtml", "text/a.xhtml", epubXHTML("<p>Root</p>")), []string{"Root"}},
		{"href with fragment", oneItemEPUB("OEBPS/content.opf", "text/a.xhtml#ch1", "OEBPS/text/a.xhtml", epubXHTML("<p>Frag</p>")), []string{"Frag"}},
		{"percent-encoded href", oneItemEPUB("OEBPS/content.opf", "text/a%20b.xhtml", "OEBPS/text/a b.xhtml", epubXHTML("<p>Space</p>")), []string{"Space"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := loadEPUB(epubZip(t, tc.entries), DefaultLimits())
			if err != nil {
				t.Fatalf("loadEPUB: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	t.Run("LoadFile on committed fixture", func(t *testing.T) {
		doc, err := LoadFile(fixtureFile(t, "sample.epub", sample))
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		want := []string{"One", "first & café", "Two second", "Three third"}
		if !reflect.DeepEqual(doc.Paragraphs, want) {
			t.Fatalf("got %q, want %q", doc.Paragraphs, want)
		}
	})
}

func TestEPUBCorrupt(t *testing.T) {
	sample := epubZip(t, sampleEPUBEntries())
	half := sample[:len(sample)/2]
	mime := epubEntry{"mimetype", []byte("application/epub+zip")}
	body := epubXHTML("<p>x</p>")
	opf := func(manifest, spine string) epubEntry {
		return epubEntry{"OEBPS/content.opf", epubOPF(manifest, spine)}
	}
	escaping := func(href, entry string) []epubEntry {
		return []epubEntry{mime, epubContainer("OEBPS/content.opf"),
			opf(xhtmlItem("a", href), `<itemref idref="a"/>`), {entry, body}}
	}
	fullOPF := epubOPF(xhtmlItem("a", "a.xhtml"), `<itemref idref="a"/>`)
	tests := []struct {
		name string
		b    []byte
	}{
		{"truncated to half", half},
		{"no container.xml", epubZip(t, []epubEntry{mime, opf(xhtmlItem("a", "a.xhtml"), `<itemref idref="a"/>`), {"OEBPS/a.xhtml", body}})},
		{"container without rootfile", epubZip(t, []epubEntry{mime,
			{"META-INF/container.xml", []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles/></container>`)},
			opf(xhtmlItem("a", "a.xhtml"), `<itemref idref="a"/>`), {"OEBPS/a.xhtml", body}})},
		{"rootfile not in zip", epubZip(t, []epubEntry{mime, epubContainer("OEBPS/missing.opf"),
			opf(xhtmlItem("a", "a.xhtml"), `<itemref idref="a"/>`), {"OEBPS/a.xhtml", body}})},
		{"OPF cut off inside a tag", epubZip(t, []epubEntry{mime, epubContainer("OEBPS/content.opf"),
			{"OEBPS/content.opf", fullOPF[:bytes.Index(fullOPF, []byte("<itemref"))+10]}, {"OEBPS/a.xhtml", body}})},
		{"idref not in manifest", epubZip(t, []epubEntry{mime, epubContainer("OEBPS/content.opf"),
			opf(xhtmlItem("a", "a.xhtml"), `<itemref idref="a"/><itemref idref="ghost"/>`), {"OEBPS/a.xhtml", body}})},
		{"only linear=no", epubZip(t, []epubEntry{mime, epubContainer("OEBPS/content.opf"),
			opf(xhtmlItem("a", "a.xhtml"), `<itemref idref="a" linear="no"/>`), {"OEBPS/a.xhtml", body}})},
		{"href ../../etc/passwd", epubZip(t, escaping("../../etc/passwd", "../../etc/passwd"))},
		{"href ../outside.xhtml", epubZip(t, escaping("../outside.xhtml", "../outside.xhtml"))},
		{"href /etc/passwd", epubZip(t, escaping("/etc/passwd", "/etc/passwd"))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := loadEPUB(tc.b, DefaultLimits())
			if !errors.Is(err, ErrCorruptEPUB) {
				t.Fatalf("loadEPUB = %q, %v; want ErrCorruptEPUB", got, err)
			}
			if strings.Contains(err.Error(), "\n") {
				t.Errorf("multi-line error: %q", err)
			}
		})
	}
	t.Run("LoadFile truncated x.epub", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "x.epub")
		if err := os.WriteFile(p, half, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadFile(p); !errors.Is(err, ErrCorruptEPUB) {
			t.Fatalf("LoadFile: %v, want ErrCorruptEPUB", err)
		}
	})
}

func TestEPUBLimits(t *testing.T) {
	mime := epubEntry{"mimetype", []byte("application/epub+zip")}
	para := func(n int) []byte {
		return epubXHTML("<p>" + strings.Repeat("a ", n/2) + "</p>")
	}
	book := func(spine string, extra ...epubEntry) []byte {
		e := []epubEntry{mime, epubContainer("OEBPS/content.opf"),
			{"OEBPS/content.opf", epubOPF(xhtmlItem("a", "a.xhtml")+xhtmlItem("b", "b.xhtml"), spine)}}
		return epubZip(t, append(e, extra...))
	}
	small := Limits{MaxBytes: 1 << 20, MaxEntries: 50, MaxDepth: 256}

	many := []epubEntry{mime, epubContainer("OEBPS/content.opf"),
		{"OEBPS/content.opf", epubOPF(xhtmlItem("a", "a.xhtml"), `<itemref idref="a"/>`)},
		{"OEBPS/a.xhtml", para(10)}}
	for i := len(many); i < 60; i++ {
		many = append(many, epubEntry{fmt.Sprintf("OEBPS/extra%02d.css", i), []byte("p{}")})
	}

	deep := strings.Repeat("<x>", 300) + strings.Repeat("</x>", 300)
	repeated := strings.Repeat(`<itemref idref="a"/>`, 2000)

	tests := []struct {
		name string
		b    []byte
		lim  Limits
	}{
		{"two 600 KiB items, 1 MiB budget", book(`<itemref idref="a"/><itemref idref="b"/>`,
			epubEntry{"OEBPS/a.xhtml", para(600 << 10)}, epubEntry{"OEBPS/b.xhtml", para(600 << 10)}), small},
		{"8 MiB zero-filled item", book(`<itemref idref="a"/>`,
			epubEntry{"OEBPS/a.xhtml", make([]byte, 8<<20)}), small},
		{"small item 2000 times", book(repeated, epubEntry{"OEBPS/a.xhtml", para(1 << 10)}),
			Limits{MaxBytes: 256 << 10, MaxEntries: 50, MaxDepth: 256}},
		{"60 entries, MaxEntries 50", epubZip(t, many), small},
		{"OPF 300 deep, MaxDepth 256", epubZip(t, []epubEntry{mime, epubContainer("OEBPS/content.opf"),
			{"OEBPS/content.opf", epubOPF(xhtmlItem("a", "a.xhtml")+deep, `<itemref idref="a"/>`)},
			{"OEBPS/a.xhtml", para(10)}}), small},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			_, err := loadEPUB(tc.b, tc.lim)
			if d := time.Since(start); d > time.Second {
				t.Errorf("took %v, want < 1s", d)
			}
			if !errors.Is(err, ErrCorruptEPUB) || !errors.Is(err, ErrTooLarge) {
				t.Fatalf("err = %v, want ErrCorruptEPUB and ErrTooLarge", err)
			}
		})
	}
	t.Run("sample with DefaultLimits", func(t *testing.T) {
		if _, err := loadEPUB(epubZip(t, sampleEPUBEntries()), DefaultLimits()); err != nil {
			t.Fatalf("loadEPUB: %v", err)
		}
	})
}
