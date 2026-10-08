package input

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	content := "one two\n\nthree"
	txt := write("book.txt", content)
	resolved, err := filepath.EvalSymlinks(txt)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(txt, link); err != nil {
		t.Fatal(err)
	}
	empty := write("empty.txt", "")
	five := write("five.txt", "12345")
	md := write("notes.md", "# Title\n\nbody")

	t.Run("text file", func(t *testing.T) {
		doc, err := LoadFile(txt)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"one two", "three"}; !reflect.DeepEqual(doc.Paragraphs, want) {
			t.Errorf("Paragraphs = %q, want %q", doc.Paragraphs, want)
		}
		if !filepath.IsAbs(doc.Path) || doc.Path != resolved {
			t.Errorf("Path = %q, want %q", doc.Path, resolved)
		}
		if doc.SHA256 != sha256.Sum256([]byte(content)) {
			t.Errorf("SHA256 = %x, want hash of content", doc.SHA256)
		}
	})

	tests := []struct {
		name  string
		load  func() (Document, error)
		errIs error
		paras []string
		path  string
	}{
		{"symlink", func() (Document, error) { return LoadFile(link) }, nil, []string{"one two", "three"}, resolved},
		{"missing", func() (Document, error) { return LoadFile(filepath.Join(dir, "nope.txt")) }, ErrNotFound, nil, ""},
		{"too large", func() (Document, error) {
			return loadFile(five, Limits{MaxBytes: 4, MaxEntries: 1, MaxDepth: 1})
		}, ErrTooLarge, nil, ""},
		{"empty", func() (Document, error) { return LoadFile(empty) }, nil, nil, ""},
		{"markdown", func() (Document, error) { return LoadFile(md) }, nil, []string{"Title", "body"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := tt.load()
			if tt.errIs != nil {
				if !errors.Is(err, tt.errIs) {
					t.Fatalf("err = %v, want %v", err, tt.errIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if !reflect.DeepEqual(doc.Paragraphs, tt.paras) {
				t.Errorf("Paragraphs = %q, want %q", doc.Paragraphs, tt.paras)
			}
			if tt.path != "" && doc.Path != tt.path {
				t.Errorf("Path = %q, want %q", doc.Path, tt.path)
			}
		})
	}

	t.Run("Load KindFile equals LoadFile", func(t *testing.T) {
		got, err1 := Load(Source{Kind: KindFile, Path: txt}, nil)
		want, err2 := LoadFile(txt)
		if err1 != nil || err2 != nil {
			t.Fatalf("errs = %v, %v", err1, err2)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Load = %+v, want %+v", got, want)
		}
	})
}
