package input

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// FileType is the detected format of a file source.
type FileType int

const (
	TypeText FileType = iota + 1
	TypeMarkdown
	TypeEPUB
	TypeFB2
	TypePDF
)

const (
	headSize   = 64 << 10 // bytes of the file passed to DetectType
	fb2Window  = 4 << 10  // FictionBook root must start within this many bytes
	magicEPUB  = "PK\x03\x04"
	magicPDF   = "%PDF-"
	fb2RootTag = "<FictionBook"
)

// DetectType picks the loader from the file name and head (the first up to 64 KiB).
// Rules: .epub/.pdf/.fb2 need their own magic; .txt/.md are text unless EPUB, PDF or FB2 magic
// matches; any other extension is detected by magic, else valid UTF-8 without NUL is text.
// Every error wraps ErrUnsupported and names the extension and the detected content.
func DetectType(name string, head []byte) (FileType, error) {
	ext := strings.ToLower(filepath.Ext(name))
	magic := magicType(head)
	mismatch := func() (FileType, error) {
		return 0, fmt.Errorf("%w: %s but content is %s", ErrUnsupported, describeExt(ext), describeContent(head, magic))
	}
	switch ext {
	case ".epub", ".pdf", ".fb2":
		want := map[string]FileType{".epub": TypeEPUB, ".pdf": TypePDF, ".fb2": TypeFB2}[ext]
		if magic == want {
			return want, nil
		}
		return mismatch()
	case ".txt", ".md":
		if magic != 0 {
			return mismatch()
		}
		if ext == ".md" {
			return TypeMarkdown, nil
		}
		return TypeText, nil
	}
	if magic != 0 {
		return magic, nil
	}
	if isText(head) {
		return TypeText, nil
	}
	return mismatch()
}

// magicType returns the type whose magic bytes head carries, or 0.
func magicType(head []byte) FileType {
	switch {
	case bytes.HasPrefix(head, []byte(magicEPUB)):
		return TypeEPUB
	case bytes.HasPrefix(head, []byte(magicPDF)):
		return TypePDF
	case isFB2(head):
		return TypeFB2
	}
	return 0
}

// isFB2 reports whether, within the first 4 KiB, after an optional UTF-8 BOM, whitespace, one
// optional <?xml ...?> declaration and any <!-- ... --> comments, the first element is
// <FictionBook followed by whitespace, '>' or '/'.
func isFB2(head []byte) bool {
	if len(head) > fb2Window {
		head = head[:fb2Window]
	}
	b := bytes.TrimPrefix(head, bom)
	b = trimXMLSpace(b)
	if bytes.HasPrefix(b, []byte("<?xml")) {
		i := bytes.Index(b, []byte("?>"))
		if i < 0 {
			return false
		}
		b = trimXMLSpace(b[i+2:])
	}
	for bytes.HasPrefix(b, []byte("<!--")) {
		i := bytes.Index(b[4:], []byte("-->"))
		if i < 0 {
			return false
		}
		b = trimXMLSpace(b[4+i+3:])
	}
	if !bytes.HasPrefix(b, []byte(fb2RootTag)) || len(b) == len(fb2RootTag) {
		return false
	}
	switch b[len(fb2RootTag)] {
	case ' ', '\t', '\r', '\n', '>', '/':
		return true
	}
	return false
}

func trimXMLSpace(b []byte) []byte {
	return bytes.TrimLeft(b, " \t\r\n")
}

// isText: valid UTF-8 without NUL; a rune cut off at the end of a full 64 KiB head is allowed.
func isText(head []byte) bool {
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	if utf8.Valid(head) {
		return true
	}
	if len(head) < headSize {
		return false
	}
	// Drop an incomplete trailing rune (at most 3 bytes) and check the rest.
	for cut := 1; cut <= 3 && cut < len(head); cut++ {
		tail := head[len(head)-cut:]
		if utf8.RuneStart(tail[0]) {
			if utf8.FullRune(tail) {
				return false
			}
			return utf8.Valid(head[:len(head)-cut])
		}
	}
	return false
}

func describeExt(ext string) string {
	if ext == "" {
		return "no extension"
	}
	return fmt.Sprintf("extension %q", ext)
}

func describeContent(head []byte, magic FileType) string {
	switch magic {
	case TypeEPUB:
		return "a zip/EPUB archive"
	case TypePDF:
		return "a PDF"
	case TypeFB2:
		return "FB2 XML"
	}
	if isText(head) {
		return "plain text"
	}
	return "binary data"
}
