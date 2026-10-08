package input

import (
	"bytes"
	"strings"
	"unicode"
)

// escBase is the start of the private-use range that holds backslash-escaped
// ASCII punctuation while the inline passes run.
const escBase = 0xE000

// loadMarkdown strips Markdown syntax and returns the readable text as
// paragraphs. Code blocks and inline code are dropped. It never fails.
func loadMarkdown(b []byte) ([]string, error) {
	b = bytes.TrimPrefix(b, bom)
	text := decodeUTF8(b)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var paras []string
	var cur []string
	emit := func(s string) {
		if s = cleanInline(s); s != "" {
			paras = append(paras, s)
		}
	}
	flush := func() {
		if len(cur) > 0 {
			emit(strings.Join(cur, " "))
			cur = nil
		}
	}

	var fenceCh byte
	fenceLen := 0
	for _, line := range strings.Split(text, "\n") {
		if fenceCh != 0 {
			if isFenceClose(line, fenceCh, fenceLen) {
				fenceCh = 0
			}
			continue
		}
		line = stripQuote(line)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent <= 3 {
			if ch, n := fenceOpen(trimmed); n > 0 {
				flush()
				fenceCh, fenceLen = ch, n
				continue
			}
			if h, ok := atxHeading(trimmed); ok {
				flush()
				emit(h)
				continue
			}
			if isRef(trimmed) {
				continue
			}
		}
		if isSetext(trimmed, '=') || (len(cur) > 0 && isSetext(trimmed, '-')) {
			if len(cur) > 0 {
				last := cur[len(cur)-1]
				cur = cur[:len(cur)-1]
				flush()
				emit(last)
				continue
			}
		}
		if isHR(trimmed) {
			flush()
			continue
		}
		if isTableSep(trimmed) {
			flush()
			continue
		}
		if strings.HasPrefix(trimmed, "|") {
			flush()
			emit(strings.ReplaceAll(trimmed, "|", " "))
			continue
		}
		if rest, ok := listItem(trimmed); ok {
			flush()
			cur = append(cur, rest)
			continue
		}
		cur = append(cur, trimmed)
	}
	flush()
	return paras, nil
}

func stripQuote(line string) string {
	for {
		t := strings.TrimLeft(line, " ")
		if !strings.HasPrefix(t, ">") {
			return line
		}
		line = strings.TrimPrefix(t[1:], " ")
	}
}

func fenceOpen(t string) (byte, int) {
	if t == "" || (t[0] != '`' && t[0] != '~') {
		return 0, 0
	}
	n := 0
	for n < len(t) && t[n] == t[0] {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	if t[0] == '`' && strings.Contains(t[n:], "`") {
		return 0, 0
	}
	return t[0], n
}

func isFenceClose(line string, ch byte, n int) bool {
	t := strings.TrimSpace(line)
	if len(t) < n {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] != ch {
			return false
		}
	}
	return true
}

func atxHeading(t string) (string, bool) {
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (n < len(t) && t[n] != ' ' && t[n] != '\t') {
		return "", false
	}
	h := strings.TrimSpace(t[n:])
	if tr := strings.TrimRight(h, "#"); tr != h && (tr == "" || tr[len(tr)-1] == ' ') {
		h = strings.TrimSpace(tr)
	}
	return h, true
}

func isRef(t string) bool {
	if !strings.HasPrefix(t, "[") {
		return false
	}
	i := strings.Index(t, "]:")
	return i > 1 && len(strings.TrimSpace(t[i+2:])) > 0
}

// isSetext reports whether t consists only of ch (an underline line).
func isSetext(t string, ch byte) bool {
	t = strings.TrimRight(t, " \t")
	if t == "" {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] != ch {
			return false
		}
	}
	return true
}

func isHR(t string) bool {
	var ch byte
	n := 0
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c != '*' && c != '-' && c != '_' {
			return false
		}
		if ch == 0 {
			ch = c
		} else if c != ch {
			return false
		}
		n++
	}
	return n >= 3
}

func isTableSep(t string) bool {
	if !strings.Contains(t, "|") || !strings.Contains(t, "-") {
		return false
	}
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case '|', '-', ':', ' ', '\t':
		default:
			return false
		}
	}
	return true
}

func listItem(t string) (string, bool) {
	if len(t) >= 2 && (t[0] == '-' || t[0] == '*' || t[0] == '+') && (t[1] == ' ' || t[1] == '\t') {
		return strings.TrimSpace(t[2:]), true
	}
	i := 0
	for i < len(t) && i < 9 && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(t) && (t[i] == '.' || t[i] == ')') && (t[i+1] == ' ' || t[i+1] == '\t') {
		return strings.TrimSpace(t[i+2:]), true
	}
	return "", false
}

// cleanInline applies the inline passes and collapses whitespace.
func cleanInline(s string) string {
	r := escapes(s)
	r = dropCode(r)
	r = links(r)
	r = tags(r)
	r = emphasis(r)
	for i, c := range r {
		if c >= escBase && c < escBase+128 {
			r[i] = c - escBase
		}
	}
	return strings.Join(strings.Fields(string(r)), " ")
}

func escapes(s string) []rune {
	in := []rune(s)
	out := make([]rune, 0, len(in))
	for i := 0; i < len(in); i++ {
		c := in[i]
		if c == '\\' && i+1 < len(in) && in[i+1] < 128 && isASCIIPunct(in[i+1]) {
			out = append(out, in[i+1]+escBase)
			i++
			continue
		}
		out = append(out, c)
	}
	return out
}

func isASCIIPunct(c rune) bool {
	return c > 32 && c < 127 && !unicode.IsLetter(c) && !unicode.IsDigit(c)
}

// dropCode removes inline code spans including their content.
func dropCode(in []rune) []rune {
	out := make([]rune, 0, len(in))
	failed := map[int]bool{}
	for i := 0; i < len(in); {
		if in[i] != '`' {
			out = append(out, in[i])
			i++
			continue
		}
		n := 0
		for i+n < len(in) && in[i+n] == '`' {
			n++
		}
		end := -1
		if !failed[n] {
			for j := i + n; j < len(in); {
				if in[j] != '`' {
					j++
					continue
				}
				m := 0
				for j+m < len(in) && in[j+m] == '`' {
					m++
				}
				if m == n {
					end = j + m
					break
				}
				j += m
			}
			if end < 0 {
				failed[n] = true
			}
		}
		if end < 0 {
			out = append(out, in[i:i+n]...)
			i += n
		} else {
			i = end
		}
	}
	return out
}

// nextIndex returns the index of the first c at or after from (cached via *cache), or -1.
func nextIndex(in []rune, from int, c rune, cache *int) int {
	if *cache == -1 {
		return -1
	}
	if *cache > from || (*cache == from && *cache < len(in) && in[*cache] == c) {
		return *cache
	}
	for j := from; j < len(in); j++ {
		if in[j] == c {
			*cache = j
			return j
		}
	}
	*cache = -1
	return -1
}

// links turns images and links into their text and removes link targets.
func links(in []rune) []rune {
	out := make([]rune, 0, len(in))
	closeC, parenC := 0, 0
	for i := 0; i < len(in); i++ {
		c := in[i]
		start := i
		if c == '!' && i+1 < len(in) && in[i+1] == '[' {
			start = i + 1
		} else if c != '[' {
			out = append(out, c)
			continue
		}
		cl := nextIndex(in, start+1, ']', &closeC)
		if cl < 0 {
			out = append(out, c)
			continue
		}
		next := cl + 1
		if next < len(in) && in[next] == '(' {
			if p := nextIndex(in, next+1, ')', &parenC); p >= 0 {
				out = append(out, in[start+1:cl]...)
				i = p
				continue
			}
		} else if next < len(in) && in[next] == '[' {
			rc := 0
			if p := nextIndex(in, next+1, ']', &rc); p >= 0 {
				out = append(out, in[start+1:cl]...)
				i = p
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// tags removes autolinks and inline HTML tags, keeping the text between them.
func tags(in []rune) []rune {
	out := make([]rune, 0, len(in))
	gtC := 0
	for i := 0; i < len(in); i++ {
		if in[i] == '<' && i+1 < len(in) && (in[i+1] == '/' || in[i+1] == '!' || unicode.IsLetter(in[i+1])) {
			if g := nextIndex(in, i+1, '>', &gtC); g >= 0 && g-i < 300 && !containsRune(in[i+1:g], '<') {
				i = g
				continue
			}
		}
		out = append(out, in[i])
	}
	return out
}

func containsRune(r []rune, c rune) bool {
	for _, x := range r {
		if x == c {
			return true
		}
	}
	return false
}

// emphasis removes runs of * and _ that wrap text.
func emphasis(in []rune) []rune {
	out := make([]rune, 0, len(in))
	for i := 0; i < len(in); {
		c := in[i]
		if c != '*' && c != '_' {
			out = append(out, c)
			i++
			continue
		}
		j := i
		for j < len(in) && in[j] == c {
			j++
		}
		prevSp := i == 0 || unicode.IsSpace(in[i-1])
		nextSp := j >= len(in) || unicode.IsSpace(in[j])
		intraword := !prevSp && !nextSp && isWordRune(in[i-1]) && isWordRune(in[j])
		if (prevSp && nextSp) || (c == '_' && intraword) {
			out = append(out, in[i:j]...)
		}
		i = j
	}
	return out
}

func isWordRune(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c)
}
