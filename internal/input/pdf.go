package input

import "fmt"

// loadPDF is a stub; a later task replaces this file.
func loadPDF(b []byte, lim Limits) ([]string, error) {
	return nil, fmt.Errorf("%w: PDF loader not implemented yet", ErrUnsupported)
}
