package input

import "fmt"

// loadEPUB is a stub; a later task replaces this file.
func loadEPUB(b []byte, lim Limits) ([]string, error) {
	return nil, fmt.Errorf("%w: EPUB loader not implemented yet", ErrUnsupported)
}
