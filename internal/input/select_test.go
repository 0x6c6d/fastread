package input

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSelectSource(t *testing.T) {
	d := t.TempDir()
	book := filepath.Join(d, "book.txt")
	if err := os.WriteFile(book, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(d, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(d, "pipe.txt")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("a", 5000)

	tests := []struct {
		name    string
		arg     string
		hasArg  bool
		tty     bool
		want    Source
		wantErr error
	}{
		{"regular file", book, true, true, Source{Kind: KindFile, Path: book}, nil},
		{"directory", sub, true, true, Source{}, ErrUnsupported},
		{"fifo", fifo, true, true, Source{}, ErrUnsupported},
		{"dot slash missing", "./nope.txt", true, true, Source{}, ErrNotFound},
		{"missing txt", "nope.txt", true, true, Source{}, ErrNotFound},
		{"missing upper pdf", "NOPE.PDF", true, true, Source{}, ErrNotFound},
		{"missing epub", "book.epub", true, true, Source{}, ErrNotFound},
		{"missing fb2", "x.fb2", true, true, Source{}, ErrNotFound},
		{"missing md", "notes.md", true, true, Source{}, ErrNotFound},
		{"tilde", "~/missing", true, true, Source{}, ErrNotFound},
		{"hidden", ".hidden", true, true, Source{}, ErrNotFound},
		{"slash", "a/b", true, true, Source{}, ErrNotFound},
		{"enotdir", filepath.Join(d, "missing", "deeper.txt"), true, true, Source{}, ErrNotFound},
		{"ellipsis", "...", true, true, Source{}, ErrNotFound},
		{"word", "hello", true, true, Source{Kind: KindRaw, Text: "hello"}, nil},
		{"sentence", "Hello wonderful world", true, true, Source{Kind: KindRaw, Text: "Hello wonderful world"}, nil},
		{"space before ext", "hello world.txt", true, true, Source{Kind: KindRaw, Text: "hello world.txt"}, nil},
		{"trailing dot", "word.", true, true, Source{Kind: KindRaw, Text: "word."}, nil},
		{"umlaut", "über", true, true, Source{Kind: KindRaw, Text: "über"}, nil},
		{"too long", long, true, true, Source{Kind: KindRaw, Text: long}, nil},
		{"newline", "a\nb/c", true, true, Source{Kind: KindRaw, Text: "a\nb/c"}, nil},
		{"no arg tty", "", false, true, Source{}, ErrNoInput},
		{"no arg pipe", "", false, false, Source{Kind: KindStdin}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type result struct {
				src Source
				err error
			}
			done := make(chan result, 1)
			go func() {
				s, err := Select(tt.arg, tt.hasArg, tt.tty)
				done <- result{s, err}
			}()
			var r result
			select {
			case r = <-done:
			case <-time.After(5 * time.Second):
				t.Fatalf("Select(%q) did not return", tt.arg)
			}
			if tt.wantErr != nil {
				if !errors.Is(r.err, tt.wantErr) {
					t.Fatalf("Select err = %v, want %v", r.err, tt.wantErr)
				}
				if strings.Contains(r.err.Error(), "\n") {
					t.Errorf("error message has a newline: %q", r.err.Error())
				}
				if tt.wantErr == ErrNotFound {
					if errors.Is(r.err, ErrUnsupported) {
						t.Errorf("ErrNotFound row also matches ErrUnsupported: %v", r.err)
					}
					if !strings.Contains(r.err.Error(), tt.arg) {
						t.Errorf("error %q does not name argument %q", r.err.Error(), tt.arg)
					}
				}
				return
			}
			if r.err != nil {
				t.Fatalf("Select err = %v", r.err)
			}
			if r.src != tt.want {
				t.Errorf("Select = %+v, want %+v", r.src, tt.want)
			}
		})
	}
}

func TestLooksLikePath(t *testing.T) {
	tests := []struct {
		arg  string
		want bool
	}{
		{"", false},
		{"/", true},
		{"a/b", true},
		{".hidden", true},
		{"...", true},
		{"~", true},
		{"~/x", true},
		{"nope.txt", true},
		{".TXT", true},
		{"file.Md", true},
		{"book.EPUB", true},
		{"x.fb2", true},
		{"doc.pdf", true},
		{"x.epubx", false},
		{"hello", false},
		{"word.", false},
		{"über", false},
		{"hello world.txt", false},
		{"a\nb/c", false},
		{"a\tb.md", false},
		{"x y.txt", false},
		{"file.text", false},
	}
	for _, tt := range tests {
		if got := LooksLikePath(tt.arg); got != tt.want {
			t.Errorf("LooksLikePath(%q) = %v, want %v", tt.arg, got, tt.want)
		}
	}
}
