package input

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMarkdownStrip(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		doc, err := LoadFile("testdata/sample.md")
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Split("Title Here Some emphasis, strong and under text with snake_case kept. "+
			"A link text and alt words here. Use now, see there. quoted line item one item two "+
			"item three Setext Heading a b c d Escaped 5 * 3 and html plus ref link too.", " ")
		got := strings.Fields(strings.Join(doc.Paragraphs, " "))
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("words = %q\nwant %q", got, want)
		}
		if doc.Paragraphs[0] != "Title Here" {
			t.Errorf("Paragraphs[0] = %q", doc.Paragraphs[0])
		}
		found := false
		for _, p := range doc.Paragraphs {
			if p == "Setext Heading" {
				found = true
			}
			for _, bad := range []string{"http", "fenced", "inline code", "|", "#", "`", "](", "**", "===", "img.png"} {
				if strings.Contains(p, bad) {
					t.Errorf("paragraph %q contains %q", p, bad)
				}
			}
		}
		if !found {
			t.Errorf("no Setext Heading paragraph in %q", doc.Paragraphs)
		}
	})

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"emphasis", "**a** _b_ ***c***", []string{"a b c"}},
		{"link title", `[x](u "t")`, []string{"x"}},
		{"double backtick", "``a ` b`` d", []string{"d"}},
		{"tilde fence", "~~~\ncode\n~~~\nafter", []string{"after"}},
		{"unclosed fence", "before\n```\ncode\nmore", []string{"before"}},
		{"escaped hash", `\# not heading`, []string{"# not heading"}},
		{"hr only", "---", nil},
		{"empty", "", nil},
		{"invalid utf8", "\xff", []string{"�"}},
		{"ref links", "[a][b] [c][] <br/>x", []string{"a c x"}},
		{"list own paragraphs", "- a\n- b\n2) c", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadMarkdown([]byte(tt.in))
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("large", func(t *testing.T) {
		in := strings.Repeat("some *text* with [a](b) and `c`\n", 100000)
		start := time.Now()
		if _, err := loadMarkdown([]byte(in)); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("took %v", d)
		}
	})
	t.Run("pathological", func(t *testing.T) {
		for _, s := range []string{strings.Repeat("[", 200000), strings.Repeat("`a ", 100000), strings.Repeat("<a", 100000)} {
			start := time.Now()
			loadMarkdown([]byte(s))
			if d := time.Since(start); d > 5*time.Second {
				t.Errorf("took %v", d)
			}
		}
	})
}
