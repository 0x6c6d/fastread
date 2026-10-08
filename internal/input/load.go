package input

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
)

// LoadFile reads path (capped at DefaultLimits().MaxBytes), detects the type and loads it.
func LoadFile(path string) (Document, error) {
	return loadFile(path, DefaultLimits())
}

// loadFile is LoadFile with explicit limits. Path is the absolute, symlink-resolved path (the
// resume key); SHA256 is the hash of the file bytes.
func loadFile(path string, lim Limits) (Document, error) {
	b, err := readFileCapped(path, lim.MaxBytes)
	if err != nil {
		return Document{}, err
	}
	head := b
	if len(head) > headSize {
		head = head[:headSize]
	}
	typ, err := DetectType(path, head)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", path, err)
	}
	var paras []string
	switch typ {
	case TypeText:
		paras = ParseText(b)
	case TypeMarkdown:
		paras, err = loadMarkdown(b)
	case TypeEPUB:
		paras, err = loadEPUB(b, lim)
	case TypeFB2:
		paras, err = loadFB2(b, lim)
	case TypePDF:
		paras, err = loadPDF(b, lim)
	default:
		err = fmt.Errorf("%w: unknown file type %d", ErrUnsupported, typ)
	}
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Document{}, fmt.Errorf("%s: %w", path, err)
	}
	return Document{Paragraphs: paras, Path: resolved, SHA256: sha256.Sum256(b)}, nil
}
