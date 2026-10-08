package input

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"two paragraphs", "a b\n\nc", []string{"a b", "c"}},
		{"bom dropped", "\xEF\xBB\xBFhi", []string{"hi"}},
		{"crlf and lone cr", "x\r\ny\r\rz", []string{"x\ny", "z"}},
		{"whitespace-only line separates", "a\n \t\nb", []string{"a", "b"}},
		{"invalid utf8", "bad\xffbyte", []string{"bad�byte"}},
		{"empty", "", nil},
		{"only blank lines", "\n\n  \n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseText([]byte(tt.in))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSelectBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "book.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		arg     string
		hasArg  bool
		tty     bool
		want    Source
		wantErr error
	}{
		{"regular file", file, true, true, Source{Kind: KindFile, Path: file}, nil},
		{"directory", dir, true, true, Source{}, ErrUnsupported},
		{"raw text", "Hello wonderful world", true, true, Source{Kind: KindRaw, Text: "Hello wonderful world"}, nil},
		{"no arg tty", "", false, true, Source{}, ErrNoInput},
		{"no arg pipe", "", false, false, Source{Kind: KindStdin}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Select(tt.arg, tt.hasArg, tt.tty)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Select err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Select err = %v", err)
			}
			if got != tt.want {
				t.Errorf("Select = %+v, want %+v", got, tt.want)
			}
		})
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestLoadRawStdin(t *testing.T) {
	tests := []struct {
		name    string
		src     Source
		stdin   interface{ Read([]byte) (int, error) }
		want    []string
		wantErr bool
		errIs   error
	}{
		{"raw", Source{Kind: KindRaw, Text: "a b\n\nc"}, nil, []string{"a b", "c"}, false, nil},
		{"stdin", Source{Kind: KindStdin}, strings.NewReader("x y"), []string{"x y"}, false, nil},
		{"stdin read error", Source{Kind: KindStdin}, errReader{}, nil, true, nil},
		{"file not implemented", Source{Kind: KindFile, Path: "x.txt"}, nil, nil, true, ErrUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Load(tt.src, tt.stdin)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load err = nil, want error")
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Fatalf("Load err = %v, want %v", err, tt.errIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load err = %v", err)
			}
			if !reflect.DeepEqual(doc.Paragraphs, tt.want) {
				t.Errorf("Paragraphs = %q, want %q", doc.Paragraphs, tt.want)
			}
			if doc.Path != "" {
				t.Errorf("Path = %q, want empty", doc.Path)
			}
			if doc.SHA256 != ([32]byte{}) {
				t.Errorf("SHA256 = %x, want zero", doc.SHA256)
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	if got, want := ErrNoText.Error(), "no extractable text (scanned PDF?)"; got != want {
		t.Errorf("ErrNoText = %q, want %q", got, want)
	}
	all := []error{ErrNotFound, ErrCorruptEPUB, ErrEmpty, ErrUnsupported, ErrNoText, ErrCorruptPDF, ErrTooLarge, ErrNoInput}
	for i, a := range all {
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Errorf("errors.Is(%v, %v) = true, want distinct", a, b)
			}
		}
	}
}
