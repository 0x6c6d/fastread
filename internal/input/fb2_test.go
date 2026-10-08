package input

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const fb2Sample = `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
<description><title-info><book-title>Hidden Title</book-title>
<annotation><p>Hidden annotation</p></annotation></title-info></description>
<body><title><p>Body Title</p></title>
<section><p>First body paragraph.</p><p>Second <emphasis>body</emphasis> paragraph.</p></section></body>
<body name="notes"><section><p>Note text.</p></section></body>
<binary id="c.jpg" content-type="image/jpeg">SGlkZGVu</binary>
</FictionBook>
`

func TestFB2BodyOnly(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		doc, err := LoadFile(fixtureFile(t, "sample.fb2", []byte(fb2Sample)))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"Body Title", "First body paragraph.", "Second body paragraph.", "Note text."}
		if !reflect.DeepEqual(doc.Paragraphs, want) {
			t.Fatalf("paragraphs = %q, want %q", doc.Paragraphs, want)
		}
		for _, p := range doc.Paragraphs {
			if strings.Contains(p, "Hidden") || strings.Contains(p, "SGlkZGVu") {
				t.Errorf("leaked text in %q", p)
			}
		}
	})

	lim := DefaultLimits()
	tmp := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(tmp, []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		in   string
		lim  Limits
		want []string
	}{
		{"prefixed", `<fb:FictionBook xmlns:fb="urn:x"><fb:body><fb:section><fb:p>hi there</fb:p></fb:section></fb:body></fb:FictionBook>`, lim, []string{"hi there"}},
		{"inline and entities", `<FictionBook><body><p>wo<emphasis>rd</emphasis> &amp; more&#160;text</p></body></FictionBook>`, lim, []string{"word & more text"}},
		{"verses", `<FictionBook><body><poem><stanza><v>line one</v><v>line two</v></stanza></poem></body></FictionBook>`, lim, []string{"line one", "line two"}},
		{"image only", `<FictionBook><body><image l:href="#a"/></body></FictionBook>`, lim, nil},
		{"no body", `<FictionBook><description><p>x</p></description></FictionBook>`, lim, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := loadFB2([]byte(tc.in), tc.lim)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("external entity", func(t *testing.T) {
		in := `<?xml version="1.0"?><!DOCTYPE FictionBook [<!ENTITY x SYSTEM "file://` + tmp + `">]><FictionBook><body><p>a &x; b</p></body></FictionBook>`
		got, _ := loadFB2([]byte(in), lim)
		if strings.Contains(strings.Join(got, "\n"), "SECRET") {
			t.Errorf("external entity resolved: %q", got)
		}
	})
	t.Run("windows-1251", func(t *testing.T) {
		_, err := loadFB2([]byte(`<?xml version="1.0" encoding="windows-1251"?><FictionBook><body><p>x</p></body></FictionBook>`), lim)
		if !errors.Is(err, ErrUnsupported) {
			t.Errorf("err = %v, want ErrUnsupported", err)
		}
	})
	t.Run("too deep", func(t *testing.T) {
		in := "<FictionBook><body>" + strings.Repeat("<section>", 300) + "<p>x</p>" + strings.Repeat("</section>", 300) + "</body></FictionBook>"
		_, err := loadFB2([]byte(in), Limits{MaxBytes: 1 << 20, MaxEntries: 10, MaxDepth: 256})
		if !errors.Is(err, ErrTooLarge) {
			t.Errorf("err = %v, want ErrTooLarge", err)
		}
	})
	t.Run("truncated", func(t *testing.T) {
		_, err := loadFB2([]byte(`<FictionBook><body><p>x</p></body><bin`), lim)
		if err == nil {
			t.Error("want error for input cut inside a tag")
		}
	})
	t.Run("performance", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("<FictionBook><body><section>")
		for sb.Len() < 1<<20 {
			sb.WriteString("<p>The quick brown fox jumps over the lazy dog &amp; friends.</p>\n")
		}
		sb.WriteString("</section></body></FictionBook>")
		limit := 2 * time.Second
		if raceEnabled {
			limit = 20 * time.Second
		}
		start := time.Now()
		got, err := loadFB2([]byte(sb.String()), lim)
		if err != nil || len(got) == 0 {
			t.Fatalf("got %d paragraphs, err %v", len(got), err)
		}
		if d := time.Since(start); d > limit {
			t.Errorf("took %v, limit %v", d, limit)
		}
	})
}
