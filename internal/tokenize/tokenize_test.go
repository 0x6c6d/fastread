package tokenize

import (
	"reflect"
	"testing"
)

func TestTokenizeBasic(t *testing.T) {
	tests := []struct {
		name       string
		paragraphs []string
		want       []Token
	}{
		{name: "nil", paragraphs: nil, want: nil},
		{name: "only whitespace", paragraphs: []string{"", "   \n\t "}, want: nil},
		{
			name:       "single paragraph",
			paragraphs: []string{"Hello wonderful world"},
			want: []Token{
				{Text: "Hello"},
				{Text: "wonderful"},
				{Text: "world", ParaEnd: true},
			},
		},
		{
			name:       "punctuation and newline",
			paragraphs: []string{"Hello, world.", "Next  para\nline"},
			want: []Token{
				{Text: "Hello,"},
				{Text: "world.", ParaEnd: true},
				{Text: "Next"},
				{Text: "para"},
				{Text: "line", ParaEnd: true},
			},
		},
		{
			name:       "unicode spaces",
			paragraphs: []string{"a b c"},
			want: []Token{
				{Text: "a"},
				{Text: "b"},
				{Text: "c", ParaEnd: true},
			},
		},
		{
			name:       "empty paragraph skipped",
			paragraphs: []string{"x", "", "y"},
			want: []Token{
				{Text: "x", ParaEnd: true},
				{Text: "y", ParaEnd: true},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokenize(tc.paragraphs)
			if len(tc.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("Tokenize(%q) = %v, want no tokens", tc.paragraphs, got)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Tokenize(%q) = %v, want %v", tc.paragraphs, got, tc.want)
			}
		})
	}
}
