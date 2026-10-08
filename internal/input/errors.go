package input

import "errors"

// Sentinel errors returned (usually wrapped with %w) by the input package.
var (
	ErrNotFound    = errors.New("file not found")
	ErrCorruptEPUB = errors.New("corrupt EPUB")
	ErrEmpty       = errors.New("empty text")
	ErrUnsupported = errors.New("unsupported file type")
	ErrNoText      = errors.New("no extractable text (scanned PDF?)")
	ErrCorruptPDF  = errors.New("corrupt PDF")
	ErrTooLarge    = errors.New("input too large")
	ErrNoInput     = errors.New("no input: give text or a file, or pipe text on stdin")
)
