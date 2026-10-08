package input

// safe.go is the single home of the untrusted-input defences of this package: every read of
// untrusted bytes, every zip archive and every XML decoder goes through these helpers.

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	MaxInputBytes = 256 << 20 // file/stdin bytes, PDF size, EPUB total decompressed bytes (A4)
	MaxZipEntries = 10000
	MaxXMLDepth   = 256
)

// Limits caps resource use while parsing untrusted input; tests pass small values.
type Limits struct {
	MaxBytes   int64
	MaxEntries int
	MaxDepth   int
}

// DefaultLimits returns {MaxInputBytes, MaxZipEntries, MaxXMLDepth}.
func DefaultLimits() Limits {
	return Limits{MaxBytes: MaxInputBytes, MaxEntries: MaxZipEntries, MaxDepth: MaxXMLDepth}
}

// readCapped reads all of r; more than max bytes -> error wrapping ErrTooLarge (reads at most
// max+1 bytes, never more). Read errors are wrapped with %w.
func readCapped(r io.Reader, max int64) ([]byte, error) {
	if max < 0 {
		max = 0
	}
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("more than %d bytes: %w", max, ErrTooLarge)
	}
	return b, nil
}

// readFileCapped opens path, rejects a size above max from Stat before reading (ErrTooLarge),
// then reads through readCapped (the file may grow). Not-exist -> error wrapping ErrNotFound.
func readFileCapped(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w", path, ErrNotFound)
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if fi.Size() > max {
		return nil, fmt.Errorf("%s: %d bytes, limit %d: %w", path, fi.Size(), max, ErrTooLarge)
	}
	b, err := readCapped(f, max)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return b, nil
}

// zipArchive is an in-memory zip whose entry names are only map keys, never filesystem paths.
// It is not safe for concurrent use.
type zipArchive struct {
	files     map[string]*zip.File
	remaining int64 // decompressed-byte budget shared by all reads
}

// openZip: zip.NewReader over b. A zip.NewReader error -> ErrCorruptEPUB (wrapped), except
// zip.ErrInsecurePath, which is ignored (names are never used as paths). More than
// lim.MaxEntries entries -> fmt.Errorf("%w: %w", ErrCorruptEPUB, ErrTooLarge).
func openZip(b []byte, lim Limits) (*zipArchive, error) {
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil && !(errors.Is(err, zip.ErrInsecurePath) && r != nil) {
		return nil, fmt.Errorf("%w: %w", ErrCorruptEPUB, err)
	}
	if len(r.File) > lim.MaxEntries {
		return nil, fmt.Errorf("%w: %d zip entries, limit %d: %w", ErrCorruptEPUB, len(r.File), lim.MaxEntries, ErrTooLarge)
	}
	z := &zipArchive{files: make(map[string]*zip.File, len(r.File)), remaining: lim.MaxBytes}
	for _, f := range r.File {
		if _, dup := z.files[f.Name]; !dup {
			z.files[f.Name] = f
		}
	}
	return z, nil
}

func (z *zipArchive) has(name string) bool {
	_, ok := z.files[name]
	return ok
}

// read returns the entry with exactly this name (first one if duplicated). Missing ->
// ErrCorruptEPUB. All reads of one archive share one budget of lim.MaxBytes decompressed bytes,
// counted on the bytes actually decompressed (io.LimitReader to remaining+1, never trusting
// header sizes; a header size above the remaining budget is rejected early). Over budget
// -> ErrCorruptEPUB and ErrTooLarge both matchable with errors.Is. Decompression/checksum errors
// -> ErrCorruptEPUB.
func (z *zipArchive) read(name string) ([]byte, error) {
	f, ok := z.files[name]
	if !ok {
		return nil, fmt.Errorf("%w: missing entry %q", ErrCorruptEPUB, name)
	}
	if z.remaining < 0 {
		z.remaining = 0
	}
	overBudget := func() error {
		return fmt.Errorf("%w: entry %q exceeds the decompressed size budget: %w", ErrCorruptEPUB, name, ErrTooLarge)
	}
	if f.UncompressedSize64 > uint64(z.remaining) {
		return nil, overBudget()
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: entry %q: %w", ErrCorruptEPUB, name, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, z.remaining+1))
	if int64(len(b)) > z.remaining {
		z.remaining = 0
		return nil, overBudget()
	}
	z.remaining -= int64(len(b))
	if err != nil {
		return nil, fmt.Errorf("%w: entry %q: %w", ErrCorruptEPUB, name, err)
	}
	return b, nil
}

// newXMLDecoder: encoding/xml with Strict=false, Entity=xml.HTMLEntity,
// AutoClose=xml.HTMLAutoClose; CharsetReader accepts only utf-8/us-ascii (any case) and returns
// an error wrapping ErrUnsupported ("unsupported XML encoding") for anything else.
// encoding/xml never resolves external entities or fetches DTDs; never add code that does.
func newXMLDecoder(r io.Reader) *xml.Decoder {
	d := xml.NewDecoder(r)
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.AutoClose = xml.HTMLAutoClose
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(charset) {
		case "utf-8", "utf8", "us-ascii", "ascii":
			return input, nil
		}
		return nil, fmt.Errorf("unsupported XML encoding %q: %w", charset, ErrUnsupported)
	}
	return d
}

// walkXML calls fn for every token (xml.CopyToken'd) with the element depth (root = 1).
// Depth above maxDepth -> error wrapping ErrTooLarge. Syntax errors are wrapped with %w.
// An error from fn stops the walk and is returned.
func walkXML(d *xml.Decoder, maxDepth int, fn func(tok xml.Token, depth int) error) error {
	depth := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("xml: %w", err)
		}
		tok = xml.CopyToken(tok)
		switch tok.(type) {
		case xml.StartElement:
			depth++
			if depth > maxDepth {
				return fmt.Errorf("xml nesting deeper than %d: %w", maxDepth, ErrTooLarge)
			}
			if err := fn(tok, depth); err != nil {
				return err
			}
		case xml.EndElement:
			if err := fn(tok, depth); err != nil {
				return err
			}
			if depth > 0 {
				depth--
			}
		default:
			if err := fn(tok, depth); err != nil {
				return err
			}
		}
	}
}

// recoverAs runs fn; a panic inside it becomes an error wrapping sentinel ("%w: internal
// parser panic: %v"). fn's own error is returned unchanged.
func recoverAs(sentinel error, fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: internal parser panic: %v", sentinel, p)
		}
	}()
	return fn()
}
