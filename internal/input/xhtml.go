package input

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// xhtmlSkip lists elements whose content is never readable text.
var xhtmlSkip = map[string]bool{
	"head": true, "script": true, "style": true, "template": true,
	"noscript": true, "title": true,
}

// xhtmlBlock lists elements whose start, end or self-closing tag ends a paragraph.
var xhtmlBlock = map[string]bool{
	"p": true, "div": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "li": true, "blockquote": true, "br": true,
	"tr": true, "section": true, "article": true, "aside": true,
	"header": true, "footer": true, "nav": true, "pre": true, "ul": true,
	"ol": true, "table": true, "dl": true, "dt": true, "dd": true,
	"figure": true, "figcaption": true, "hr": true, "body": true,
}

// xhtmlCell lists table cells: their tags separate words without ending the paragraph.
var xhtmlCell = map[string]bool{"td": true, "th": true}

// xhtmlParagraphs extracts the readable text of an XHTML/HTML document as paragraphs.
func xhtmlParagraphs(b []byte) []string {
	var (
		paras []string
		cur   strings.Builder
		skip  int
	)
	flush := func() {
		if p := strings.Join(strings.Fields(cur.String()), " "); p != "" {
			paras = append(paras, p)
		}
		cur.Reset()
	}
	z := html.NewTokenizer(bytes.NewReader(b))
	for {
		switch z.Next() {
		case html.ErrorToken:
			flush()
			return paras
		case html.TextToken:
			if skip == 0 {
				cur.WriteString(strings.ToValidUTF8(string(z.Text()), "�"))
			}
		case html.StartTagToken:
			name, _ := z.TagName()
			n := string(name)
			if xhtmlSkip[n] {
				skip++
			}
			if xhtmlBlock[n] {
				flush()
			} else if xhtmlCell[n] {
				cur.WriteByte(' ')
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			n := string(name)
			if xhtmlSkip[n] && skip > 0 {
				skip--
			}
			if xhtmlBlock[n] {
				flush()
			} else if xhtmlCell[n] {
				cur.WriteByte(' ')
			}
		case html.SelfClosingTagToken:
			// A self-closing raw-text tag (e.g. <script src="a.js"/>) must not
			// switch the tokenizer into raw-text mode; it never changes skip.
			z.NextIsNotRawText()
			name, _ := z.TagName()
			if xhtmlBlock[string(name)] {
				flush()
			}
		}
	}
}
