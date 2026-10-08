package input

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestXHTMLText(t *testing.T) {
	full := `<?xml version="1.0" encoding="utf-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Head Title</title><style>p{color:red}</style></head><body><h1>One</h1><p>first &amp; caf&eacute;</p><script>var hidden = 1;</script></body></html>`
	nested := strings.Repeat("<div>", 1000) + "deep" + strings.Repeat("</div>", 1000)
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"two paragraphs", `<p>a</p><p>b</p>`, []string{"a", "b"}},
		{"br splits", `<p>x<br/>y</p>`, []string{"x", "y"}},
		{"div tail", `<div><p>one</p>two</div>`, []string{"one", "two"}},
		{"heading", `<h2>Head</h2>text`, []string{"Head", "text"}},
		{"full document", full, []string{"One", "first & café"}},
		{"self-closing raw text", `<head><title/></head><body><script src="a.js"/><p>visible</p></body>`, []string{"visible"}},
		{"inline tags", `<p>wo<em>rd</em> and <a href="x">link</a></p>`, []string{"word and link"}},
		{"table", `<table><tr><td>a</td><td>b</td></tr><tr><td>c</td></tr></table>`, []string{"a b", "c"}},
		{"entities", `<p>&#8217;q&nbsp;r</p>`, []string{"’q r"}},
		{"comment", `<!-- hidden comment --><p>shown</p>`, []string{"shown"}},
		{"noscript", `<noscript>no</noscript><p>yes</p>`, []string{"yes"}},
		{"empty", ``, nil},
		{"invalid utf8", "\xff<p>ok</p>", []string{"�", "ok"}},
		{"unclosed p", `<p>tail`, []string{"tail"}},
		{"unclosed script", `<script>never closed`, nil},
		{"deep nesting", nested, []string{"deep"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := xhtmlParagraphs([]byte(tc.in))
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("xhtmlParagraphs(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	t.Run("performance", func(t *testing.T) {
		var sb strings.Builder
		para := "<p>The quick brown fox jumps over the lazy dog &amp; friends.</p>\n"
		for sb.Len() < 2<<20 {
			sb.WriteString(para)
		}
		doc := []byte(sb.String())
		limit := 2 * time.Second
		if raceEnabled {
			limit = 20 * time.Second
		}
		start := time.Now()
		got := xhtmlParagraphs(doc)
		if d := time.Since(start); d > limit {
			t.Errorf("converting %d bytes took %v, limit %v", len(doc), d, limit)
		}
		if len(got) == 0 || got[0] != "The quick brown fox jumps over the lazy dog & friends." {
			t.Errorf("unexpected first paragraph: %q", got[:min(1, len(got))])
		}
	})
}
