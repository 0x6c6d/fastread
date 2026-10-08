package tokenize

import (
	"fmt"
	"reflect"
	"testing"
	"unicode"
	"unicode/utf8"
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

// tokensEqual compares token slices, treating nil and empty as equal.
func tokensEqual(a, b []Token) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// w is a word token, pe a word token ending its paragraph.
func w(s string) Token  { return Token{Text: s} }
func pe(s string) Token { return Token{Text: s, ParaEnd: true} }

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []Token
	}{
		{"blank line LF", []string{"a\n\nb"}, []Token{pe("a"), pe("b")}},
		{"blank line CRLF", []string{"a\r\n\r\nb"}, []Token{pe("a"), pe("b")}},
		{"blank line CR", []string{"a\r\rb"}, []Token{pe("a"), pe("b")}},
		{"whitespace-only line", []string{"a\n \t\nb c"}, []Token{pe("a"), w("b"), pe("c")}},
		{"single newline", []string{"a\nb"}, []Token{w("a"), pe("b")}},
		{"two paragraphs", []string{"a", "b"}, []Token{pe("a"), pe("b")}},
		{"one paragraph", []string{"a b"}, []Token{w("a"), pe("b")}},
		{"paragraph separator", []string{"a b"}, []Token{pe("a"), pe("b")}},
		{"line separator is space", []string{"a b"}, []Token{w("a"), pe("b")}},
		{"control-only fields dropped", []string{"x \x1b \x07"}, []Token{pe("x")}},
		{"control-only paragraph", []string{"one two", "\x1b\x1b"}, []Token{w("one"), pe("two")}},
		{
			"punctuation attached",
			[]string{"Hello, world.", "(next) «para»"},
			[]Token{w("Hello,"), pe("world."), w("(next)"), pe("«para»")},
		},
		{"unicode spaces only", []string{" 　"}, nil},
		{
			"unicode words",
			[]string{"naïve café 日本 👩‍👩‍👧"},
			[]Token{w("naïve"), w("café"), w("日本"), pe("👩‍👩‍👧")},
		},
		{"NEL splits", []string{"a\u0085b"}, []Token{w("a"), pe("b")}},
		{"ANSI escape", []string{"\x1b[31mred\x1b[0m"}, []Token{pe("[31mred[0m")}},
		{"NUL inside word", []string{"a\x00b"}, []Token{pe("ab")}},
		{"bidi override inside word", []string{"x‮y"}, []Token{pe("xy")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokenize(tc.in)
			if !tokensEqual(got, tc.want) {
				t.Fatalf("Tokenize(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// isRemoved is the reference definition of the runes Sanitize removes.
func isRemoved(r rune) bool {
	return (r >= 0 && r <= 0x1F) || (r >= 0x7F && r <= 0x9F) ||
		(r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069)
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"hello", "hello"},
		{"\x1b[31mred\x1b[0m", "[31mred[0m"},
		{"\x1b]0;title\x07x", "]0;titlex"},
		{"a\u009b2Jb", "a2Jb"},
		{"\x9b", "�"},
		{"a\xffb", "a�b"},
		{"\x00\x01\x1f\x7f", ""},
		{"‪‫‬‭‮⁦⁧⁨⁩", ""},
		{"x‮y⁦z⁩", "xyz"},
		{"é", "é"},
		{"é", "é"},
		{"👩‍👩‍👧", "👩‍👩‍👧"},
		{"‎", "‎"},
		{"�", "�"},
		{" ", " "},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.in), func(t *testing.T) {
			got := Sanitize(tc.in)
			if got != tc.want {
				t.Fatalf("Sanitize(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("Sanitize(%q) = %q is not valid UTF-8", tc.in, got)
			}
		})
	}
	t.Run("exhaustive", func(t *testing.T) {
		for r := rune(0); r <= unicode.MaxRune; r++ {
			if r >= 0xD800 && r <= 0xDFFF {
				continue
			}
			s := string(r)
			want := s
			if isRemoved(r) {
				want = ""
			}
			if got := Sanitize(s); got != want {
				t.Fatalf("Sanitize(%U) = %q, want %q", r, got, want)
			}
		}
	})
	t.Run("no allocation when unchanged", func(t *testing.T) {
		s := "naïve café 日本"
		if n := testing.AllocsPerRun(100, func() { _ = Sanitize(s) }); n != 0 {
			t.Fatalf("Sanitize allocated %v times on clean input", n)
		}
	})
}

func FuzzTokenize(f *testing.F) {
	for _, s := range []string{"", "a b\n\nc", "\x1b[2J\x1b]0;x\x07", "\x9b31m", "‮x⁦", "a\r\n\r\n b"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		toks := Tokenize([]string{s})
		for _, tok := range toks {
			if tok.Text == "" {
				t.Fatalf("empty token from %q", s)
			}
			if !utf8.ValidString(tok.Text) {
				t.Fatalf("invalid UTF-8 token %q from %q", tok.Text, s)
			}
			for _, r := range tok.Text {
				if isRemoved(r) || unicode.IsSpace(r) {
					t.Fatalf("token %q from %q contains %U", tok.Text, s, r)
				}
			}
		}
		if len(toks) > 0 && !toks[len(toks)-1].ParaEnd {
			t.Fatalf("last token of %q lacks ParaEnd: %v", s, toks)
		}
	})
}
