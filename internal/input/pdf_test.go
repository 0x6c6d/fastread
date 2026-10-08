package input

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func samplePDF() []byte {
	return buildPDF([]string{"Hello PDF world.", "Second page here."})
}

// pdfWords returns strings.Fields of each paragraph.
func pdfWords(paras []string) [][]string {
	out := make([][]string, len(paras))
	for i, p := range paras {
		out[i] = strings.Fields(p)
	}
	return out
}

func TestPDFText(t *testing.T) {
	var thirty []string
	var thirtyWant [][]string
	for i := 1; i <= 30; i++ {
		thirty = append(thirty, fmt.Sprintf("Page number %d.", i))
		thirtyWant = append(thirtyWant, []string{"Page", "number", fmt.Sprintf("%d.", i)})
	}
	tests := []struct {
		name  string
		pages []string
		want  [][]string
	}{
		{"sample", []string{"Hello PDF world.", "Second page here."},
			[][]string{{"Hello", "PDF", "world."}, {"Second", "page", "here."}}},
		{"empty middle page", []string{"First page.", "", "Third page."},
			[][]string{{"First", "page."}, {"Third", "page."}}},
		{"escaped parentheses", []string{"a (b) c"}, [][]string{{"a", "(b)", "c"}}},
		{"30 pages", thirty, thirtyWant},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := loadPDF(buildPDF(tc.pages), DefaultLimits())
			if err != nil {
				t.Fatalf("loadPDF: %v", err)
			}
			if w := pdfWords(got); !reflect.DeepEqual(w, tc.want) {
				t.Fatalf("got %q, want %q", w, tc.want)
			}
		})
	}
	t.Run("LoadFile on committed fixture", func(t *testing.T) {
		doc, err := LoadFile(fixtureFile(t, "sample.pdf", samplePDF()))
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}
		want := [][]string{{"Hello", "PDF", "world."}, {"Second", "page", "here."}}
		if w := pdfWords(doc.Paragraphs); !reflect.DeepEqual(w, want) {
			t.Fatalf("got %q, want %q", w, want)
		}
	})
}

func TestPDFNoText(t *testing.T) {
	check := func(t *testing.T, err error) {
		t.Helper()
		if !errors.Is(err, ErrNoText) {
			t.Fatalf("err = %v, want ErrNoText", err)
		}
		if errors.Is(err, ErrCorruptPDF) {
			t.Fatalf("err = %v, must not be ErrCorruptPDF", err)
		}
		if !strings.Contains(err.Error(), "no extractable text (scanned PDF?)") {
			t.Fatalf("err = %q, want it to mention no extractable text", err)
		}
	}
	tests := []struct {
		name  string
		pages []string
	}{
		{"single empty page", []string{""}},
		{"only spaces", []string{"   "}},
		{"two empty pages", []string{"", ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadPDF(buildPDF(tc.pages), DefaultLimits())
			check(t, err)
		})
	}
	t.Run("LoadFile on committed fixture", func(t *testing.T) {
		_, err := LoadFile(fixtureFile(t, "notext.pdf", buildPDF([]string{""})))
		check(t, err)
	})
}

func TestPDFBasicCorrupt(t *testing.T) {
	sample := samplePDF()
	tests := []struct {
		name string
		b    []byte
	}{
		{"header only", []byte("%PDF-1.4\n")},
		{"sample cut in half", sample[:len(sample)/2]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadPDF(tc.b, DefaultLimits())
			if !errors.Is(err, ErrCorruptPDF) {
				t.Fatalf("err = %v, want ErrCorruptPDF", err)
			}
		})
	}
}
