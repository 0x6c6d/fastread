package input

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var errBoom = errors.New("boom")

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errBoom }

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestSafeReadCapped(t *testing.T) {
	tests := []struct {
		name    string
		r       io.Reader
		max     int64
		wantLen int
		errIs   error
	}{
		{"max-1", strings.NewReader(strings.Repeat("a", 9)), 10, 9, nil},
		{"max", strings.NewReader(strings.Repeat("a", 10)), 10, 10, nil},
		{"max+1", strings.NewReader(strings.Repeat("a", 11)), 10, 0, ErrTooLarge},
		{"read error", failingReader{}, 10, 0, errBoom},
		{"endless", zeroReader{}, 1 << 20, 0, ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := readCapped(tt.r, tt.max)
			if tt.errIs != nil {
				if !errors.Is(err, tt.errIs) {
					t.Fatalf("err = %v, want %v", err, tt.errIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if len(b) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(b), tt.wantLen)
			}
		})
	}

	dir := t.TempDir()
	big := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(big, []byte("12345678901"), 0o600); err != nil {
		t.Fatal(err)
	}
	ok := filepath.Join(dir, "ok.txt")
	if err := os.WriteFile(ok, []byte("1234567890"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := []struct {
		name  string
		path  string
		errIs error
	}{
		{"missing", filepath.Join(dir, "missing.txt"), ErrNotFound},
		{"too large", big, ErrTooLarge},
		{"at limit", ok, nil},
	}
	for _, tt := range files {
		t.Run("file "+tt.name, func(t *testing.T) {
			b, err := readFileCapped(tt.path, 10)
			if tt.errIs != nil {
				if !errors.Is(err, tt.errIs) {
					t.Fatalf("err = %v, want %v", err, tt.errIs)
				}
				return
			}
			if err != nil || string(b) != "1234567890" {
				t.Fatalf("got %q, %v", b, err)
			}
		})
	}

	want := Limits{MaxBytes: 256 << 20, MaxEntries: 10000, MaxDepth: 256}
	if got := DefaultLimits(); got != want {
		t.Errorf("DefaultLimits() = %+v, want %+v", got, want)
	}
}

type zipEntry struct {
	name string
	data []byte
}

func buildZip(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		f, err := w.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// lyingZip has one deflate entry whose header claims 10 bytes but expands to 1 MiB.
func lyingZip(t *testing.T) []byte {
	t.Helper()
	plain := make([]byte, 1<<20)
	var comp bytes.Buffer
	fw, err := flate.NewWriter(&comp, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.CreateRaw(&zip.FileHeader{
		Name:               "bomb",
		Method:             zip.Deflate,
		CRC32:              crc32.ChecksumIEEE(plain[:10]),
		CompressedSize64:   uint64(comp.Len()),
		UncompressedSize64: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(comp.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSafeZip(t *testing.T) {
	valid := buildZip(t, []zipEntry{{"mimetype", []byte("application/epub+zip")}, {"a/b.xhtml", []byte("<p>hi</p>")}})
	var many []zipEntry
	for i := 0; i < 60; i++ {
		many = append(many, zipEntry{fmt.Sprintf("f%d", i), []byte("x")})
	}
	twoBig := buildZip(t, []zipEntry{{"one", make([]byte, 40<<10)}, {"two", make([]byte, 40<<10)}})
	evil := buildZip(t, []zipEntry{{"../../etc/passwd", []byte("root:x:0:0")}})

	opens := []struct {
		name   string
		b      []byte
		lim    Limits
		errIs  []error
		reads  []string // read in order; only the last may fail
		want   string   // content of the last successful read when no error expected
		readIs []error
	}{
		{name: "valid round trip", b: valid, lim: DefaultLimits(), reads: []string{"mimetype", "a/b.xhtml"}, want: "<p>hi</p>"},
		{name: "truncated", b: valid[:len(valid)/2], lim: DefaultLimits(), errIs: []error{ErrCorruptEPUB}},
		{name: "PK prefix garbage", b: append([]byte("PK\x03\x04"), bytes.Repeat([]byte("junk"), 64)...), lim: DefaultLimits(), errIs: []error{ErrCorruptEPUB}},
		{name: "too many entries", b: buildZip(t, many), lim: Limits{MaxBytes: 1 << 20, MaxEntries: 50, MaxDepth: 10}, errIs: []error{ErrCorruptEPUB, ErrTooLarge}},
		{name: "shared budget", b: twoBig, lim: Limits{MaxBytes: 64 << 10, MaxEntries: 50, MaxDepth: 10}, reads: []string{"one", "two"}, readIs: []error{ErrCorruptEPUB, ErrTooLarge}},
		{name: "lying header", b: lyingZip(t), lim: DefaultLimits(), reads: []string{"bomb"}, readIs: []error{ErrCorruptEPUB}},
		{name: "lying header small budget", b: lyingZip(t), lim: Limits{MaxBytes: 64 << 10, MaxEntries: 50, MaxDepth: 10}, reads: []string{"bomb"}, readIs: []error{ErrCorruptEPUB}},
		{name: "traversal name", b: evil, lim: DefaultLimits(), reads: []string{"../../etc/passwd"}, want: "root:x:0:0"},
		{name: "missing name", b: valid, lim: DefaultLimits(), reads: []string{"nope"}, readIs: []error{ErrCorruptEPUB}},
	}
	for _, tt := range opens {
		t.Run(tt.name, func(t *testing.T) {
			wd, _ := os.Getwd()
			before, _ := os.ReadDir(wd)
			z, err := openZip(tt.b, tt.lim)
			if len(tt.errIs) > 0 {
				for _, s := range tt.errIs {
					if !errors.Is(err, s) {
						t.Fatalf("openZip err = %v, want %v", err, s)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("openZip err = %v", err)
			}
			var got []byte
			for i, name := range tt.reads {
				last := i == len(tt.reads)-1
				if !last || len(tt.readIs) == 0 {
					if !z.has(name) {
						t.Fatalf("has(%q) = false", name)
					}
				}
				got, err = z.read(name)
				if last && len(tt.readIs) > 0 {
					if err == nil {
						t.Fatalf("read(%q) err = nil, want %v", name, tt.readIs)
					}
					for _, s := range tt.readIs {
						if !errors.Is(err, s) {
							t.Fatalf("read(%q) err = %v, want %v", name, err, s)
						}
					}
					return
				}
				if err != nil {
					t.Fatalf("read(%q) err = %v", name, err)
				}
			}
			if string(got) != tt.want {
				t.Errorf("read = %q, want %q", got, tt.want)
			}
			after, _ := os.ReadDir(wd)
			if len(after) != len(before) {
				t.Errorf("working directory changed: %d -> %d entries", len(before), len(after))
			}
			if _, err := os.Stat(filepath.Join(wd, "..", "..", "etc", "passwd")); err == nil {
				t.Error("zip entry was written to disk")
			}
		})
	}
}

func TestSafeXML(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	xxe := `<?xml version="1.0"?><!DOCTYPE r [<!ENTITY x SYSTEM "file://` + secret + `">]><r>&x;</r>`
	tests := []struct {
		name     string
		doc      string
		maxDepth int
		errIs    error
		wantErr  bool
		wantText string // expected concatenated char data (when non-empty)
		notText  string // must not appear in char data
	}{
		{name: "external entity", doc: xxe, maxDepth: 256, notText: "SECRET"},
		{name: "html entities", doc: `<r>a &amp; &eacute;</r>`, maxDepth: 256, wantText: "a & é"},
		{name: "unsupported encoding", doc: `<?xml version="1.0" encoding="windows-1251"?><r>x</r>`, maxDepth: 256, errIs: ErrUnsupported},
		{name: "us-ascii accepted", doc: `<?xml version="1.0" encoding="US-ASCII"?><r>ok</r>`, maxDepth: 256, wantText: "ok"},
		{name: "nesting 300", doc: strings.Repeat("<a>", 300) + strings.Repeat("</a>", 300), maxDepth: 256, errIs: ErrTooLarge},
		{name: "nesting 200", doc: strings.Repeat("<a>", 200) + "deep" + strings.Repeat("</a>", 200), maxDepth: 256, wantText: "deep"},
		{name: "truncated", doc: `<r><p>text</p><p`, maxDepth: 256, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var text strings.Builder
			maxSeen := 0
			err := recoverAs(errBoom, func() error {
				return walkXML(newXMLDecoder(strings.NewReader(tt.doc)), tt.maxDepth, func(tok xml.Token, depth int) error {
					if depth > maxSeen {
						maxSeen = depth
					}
					if cd, ok := tok.(xml.CharData); ok {
						text.Write(cd)
					}
					return nil
				})
			})
			if errors.Is(err, errBoom) {
				t.Fatalf("panic: %v", err)
			}
			if tt.errIs != nil || tt.wantErr {
				if err == nil {
					t.Fatal("err = nil, want error")
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Fatalf("err = %v, want %v", err, tt.errIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if tt.wantText != "" && text.String() != tt.wantText {
				t.Errorf("text = %q, want %q", text.String(), tt.wantText)
			}
			if tt.notText != "" && strings.Contains(text.String(), tt.notText) {
				t.Errorf("text %q contains %q", text.String(), tt.notText)
			}
			if tt.name == "nesting 200" && maxSeen != 200 {
				t.Errorf("max depth = %d, want 200", maxSeen)
			}
		})
	}

	t.Run("fn error stops walk", func(t *testing.T) {
		n := 0
		err := walkXML(newXMLDecoder(strings.NewReader(`<r><a/><b/></r>`)), 256, func(xml.Token, int) error {
			n++
			return errBoom
		})
		if !errors.Is(err, errBoom) || n != 1 {
			t.Errorf("err = %v, calls = %d; want errBoom, 1", err, n)
		}
	})
}

func TestSafeRecover(t *testing.T) {
	tests := []struct {
		name  string
		fn    func() error
		errIs []error
		same  error
	}{
		{"panic string", func() error { panic("bad xref") }, []error{ErrCorruptPDF}, nil},
		{"panic error", func() error { panic(io.ErrUnexpectedEOF) }, []error{ErrCorruptPDF}, nil},
		{"fn error", func() error { return errBoom }, nil, errBoom},
		{"nil", func() error { return nil }, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := recoverAs(ErrCorruptPDF, tt.fn)
			if tt.errIs != nil {
				for _, s := range tt.errIs {
					if !errors.Is(err, s) {
						t.Fatalf("err = %v, want %v", err, s)
					}
				}
				if !strings.Contains(err.Error(), "internal parser panic") {
					t.Errorf("err = %q, want 'internal parser panic'", err)
				}
				return
			}
			if err != tt.same {
				t.Errorf("err = %v, want %v", err, tt.same)
			}
		})
	}
}

func TestSafeLoadStdinCap(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		errIs error
		want  []string
	}{
		{"over cap", "123456789", ErrTooLarge, nil},
		{"at cap", "12345678", nil, []string{"12345678"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := load(Source{Kind: KindStdin}, strings.NewReader(tt.stdin), Limits{MaxBytes: 8})
			if tt.errIs != nil {
				if !errors.Is(err, tt.errIs) {
					t.Fatalf("err = %v, want %v", err, tt.errIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if !reflect.DeepEqual(doc.Paragraphs, tt.want) {
				t.Errorf("Paragraphs = %q, want %q", doc.Paragraphs, tt.want)
			}
		})
	}
}
