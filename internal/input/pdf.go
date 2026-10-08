package input

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// MaxPDFPages caps the number of pages whose text is extracted; later pages are ignored.
const MaxPDFPages = 10000

const (
	maxPDFTreeDepth = 32     // page-tree nesting limit
	maxPDFTreeNodes = 100000 // page-tree nodes visited at most
)

// loadPDF extracts the text layer of a PDF, one or more paragraphs per page, in page order.
// This file is the only user of github.com/ledongthuc/pdf; every library call runs inside a
// single recoverAs wrapper because the library panics on malformed input. The page tree is
// walked here with explicit depth and node limits instead of trusting the /Count value.
func loadPDF(b []byte, lim Limits) ([]string, error) {
	if int64(len(b)) > lim.MaxBytes {
		return nil, fmt.Errorf("PDF of %d bytes, limit %d: %w", len(b), lim.MaxBytes, ErrTooLarge)
	}
	var paras []string
	err := recoverAs(ErrCorruptPDF, func() error {
		r, err := pdf.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			if errors.Is(err, pdf.ErrInvalidPassword) {
				return fmt.Errorf("%w: encrypted PDF not supported", ErrUnsupported)
			}
			return fmt.Errorf("%w: %v", ErrCorruptPDF, err)
		}
		pages, err := pdfPages(r.Trailer().Key("Root").Key("Pages"))
		if err != nil {
			return err
		}
		var total int64
		for i, p := range pages {
			text, err := p.GetPlainText(nil)
			if err != nil {
				return fmt.Errorf("%w: page %d: %v", ErrCorruptPDF, i+1, err)
			}
			total += int64(len(text))
			if total > lim.MaxBytes {
				return fmt.Errorf("PDF text exceeds %d bytes: %w", lim.MaxBytes, ErrTooLarge)
			}
			paras = append(paras, ParseText([]byte(text))...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, p := range paras {
		if strings.IndexFunc(p, func(r rune) bool { return !unicode.IsSpace(r) }) >= 0 {
			return paras, nil
		}
	}
	return nil, ErrNoText
}

// pdfPages walks the page tree depth-first over Kids in order and returns at most MaxPDFPages
// leaf pages. Nesting deeper than maxPDFTreeDepth or more than maxPDFTreeNodes visited nodes
// -> ErrCorruptPDF (this also stops self-referencing trees). Must run inside recoverAs.
func pdfPages(root pdf.Value) ([]pdf.Page, error) {
	var pages []pdf.Page
	visited := 0
	var walk func(node pdf.Value, depth int) error
	walk = func(node pdf.Value, depth int) error {
		if depth > maxPDFTreeDepth {
			return fmt.Errorf("%w: page tree deeper than %d", ErrCorruptPDF, maxPDFTreeDepth)
		}
		kids := node.Key("Kids")
		n := kids.Len()
		for i := 0; i < n && len(pages) < MaxPDFPages; i++ {
			visited++
			if visited > maxPDFTreeNodes {
				return fmt.Errorf("%w: page tree has more than %d nodes", ErrCorruptPDF, maxPDFTreeNodes)
			}
			kid := kids.Index(i)
			switch kid.Key("Type").Name() {
			case "Pages":
				if err := walk(kid, depth+1); err != nil {
					return err
				}
			case "Page":
				pages = append(pages, pdf.Page{V: kid})
			}
		}
		return nil
	}
	if err := walk(root, 1); err != nil {
		return nil, err
	}
	return pages, nil
}
