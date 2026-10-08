package input

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectType(t *testing.T) {
	fb2Head := "<?xml version=\"1.0\"?>\n<!-- c\n--><FictionBook xmlns=\"http://www.gribuser.ru/xml/fictionbook/2.0\">"
	// 65536 bytes ending with the first byte of a 2-byte rune (é = C3 A9).
	cut := strings.Repeat("a", headSize-1) + "\xC3"
	tests := []struct {
		name    string
		file    string
		head    string
		want    FileType
		wantErr bool
	}{
		{"txt", "a.txt", "hello world", TypeText, false},
		{"upper TXT", "A.TXT", "hello", TypeText, false},
		{"mixed Md", "n.Md", "# title", TypeMarkdown, false},
		{"epub", "b.epub", "PK\x03\x04rest", TypeEPUB, false},
		{"pdf", "d.pdf", "%PDF-1.4\n", TypePDF, false},
		{"fb2 decl and comment", "x.fb2", fb2Head, TypeFB2, false},
		{"fb2 bom", "x.fb2", "\xEF\xBB\xBF<FictionBook>", TypeFB2, false},
		{"noext epub", "book", "PK\x03\x04rest", TypeEPUB, false},
		{"noext pdf", "book", "%PDF-1.7", TypePDF, false},
		{"noext fb2", "book", fb2Head, TypeFB2, false},
		{"noext utf8", "book", "Grüße, world", TypeText, false},
		{"cut rune at 64 KiB", "book", cut, TypeText, false},
		{"fb2 root too late", "x.FB2", strings.Repeat(" ", 5000) + "<FictionBook>", 0, true},
		{"FictionBookX is not fb2", "x.fb2", "<FictionBookX>", 0, true},
		{"noext binary", "book", "a\x00b", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectType(tt.file, []byte(tt.head))
			if tt.wantErr {
				if !errors.Is(err, ErrUnsupported) {
					t.Fatalf("DetectType err = %v, want ErrUnsupported", err)
				}
				if strings.Contains(err.Error(), "\n") {
					t.Errorf("error %q spans several lines", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectType err = %v", err)
			}
			if got != tt.want {
				t.Errorf("DetectType = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUnsupportedMismatch(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		file    string
		content string
	}{
		{"x.pdf", "hello world"},
		{"x.epub", "just some text"},
		{"x.fb2", "<?xml version=\"1.0\"?><html></html>"},
		{"x.txt", "%PDF-1.4\n"},
		{"x.md", "PK\x03\x04 zip"},
		{"noext", "a\x00b"},
		{"x.dat", "bad \xff\xfe utf8"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			p := filepath.Join(dir, tt.file)
			if err := os.WriteFile(p, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadFile(p)
			if !errors.Is(err, ErrUnsupported) {
				t.Fatalf("LoadFile(%s) err = %v, want ErrUnsupported", tt.file, err)
			}
			if msg := err.Error(); strings.Contains(msg, "\n") || !strings.Contains(msg, "content is") {
				t.Errorf("error %q: want one line naming the content", msg)
			}
		})
	}
}
