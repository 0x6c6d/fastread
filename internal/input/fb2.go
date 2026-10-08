package input

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// fb2Breaks are the elements whose start and end both end a paragraph.
var fb2Breaks = map[string]bool{
	"p": true, "v": true, "subtitle": true, "title": true, "text-author": true, "date": true,
	"empty-line": true, "section": true, "epigraph": true, "poem": true, "stanza": true,
	"cite": true, "table": true, "tr": true,
}

// loadFB2 extracts the text of every body element of an FB2 document as paragraphs.
// description, binary and anything outside a body are ignored.
func loadFB2(b []byte, lim Limits) ([]string, error) {
	var paras []string
	var cur strings.Builder
	bodies := 0
	flush := func() {
		if s := strings.Join(strings.Fields(cur.String()), " "); s != "" {
			paras = append(paras, s)
		}
		cur.Reset()
	}
	d := newXMLDecoder(bytes.NewReader(b))
	err := walkXML(d, lim.MaxDepth, func(tok xml.Token, depth int) error {
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "body" {
				bodies++
			} else if bodies > 0 && fb2Breaks[t.Name.Local] {
				flush()
			}
		case xml.EndElement:
			if t.Name.Local == "body" {
				flush()
				if bodies > 0 {
					bodies--
				}
			} else if bodies > 0 && fb2Breaks[t.Name.Local] {
				flush()
			}
		case xml.CharData:
			if bodies > 0 {
				cur.Write(t)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid FB2: %w", err)
	}
	flush()
	return paras, nil
}
