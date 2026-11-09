package parser

import (
	"regexp"
	"strings"
)

// Marker separates the cue from the answer in a fact sentence.
const Marker = "::"

var boldRe = regexp.MustCompile(`\*\*([^*]+?)\*\*`)

// maskCode returns a copy of s in which every inline code span (`...`,
// ``...``, ...) is replaced byte-for-byte with NUL characters. Indices in the
// result match indices in s, so markup found in the mask can be located in s
// while anything inside code is ignored.
func maskCode(s string) string {
	b := []byte(s)
	i := 0
	for i < len(b) {
		if b[i] != '`' {
			i++
			continue
		}
		n := 0
		for i+n < len(b) && b[i+n] == '`' {
			n++
		}
		end := closingRun(s, i+n, n)
		if end < 0 {
			// Unclosed run: the backticks are literal.
			i += n
			continue
		}
		for j := i; j < end+n; j++ {
			b[j] = 0
		}
		i = end + n
	}
	return string(b)
}

// closingRun returns the index of the next run of exactly n backticks at or
// after start, or -1 if none exists.
func closingRun(s string, start, n int) int {
	i := start
	for i < len(s) {
		if s[i] != '`' {
			i++
			continue
		}
		m := 0
		for i+m < len(s) && s[i+m] == '`' {
			m++
		}
		if m == n {
			return i
		}
		i += m
	}
	return -1
}

// SplitMarker splits a fact on the Marker, ignoring occurrences inside code
// spans. It returns the text before and after the first marker, trimmed, and
// the number of markers found. When count is 0, before is the whole (trimmed)
// fact and after is empty.
func SplitMarker(fact string) (before, after string, count int) {
	masked := maskCode(fact)
	count = strings.Count(masked, Marker)
	if count == 0 {
		return strings.TrimSpace(fact), "", 0
	}
	first := strings.Index(masked, Marker)
	return strings.TrimSpace(fact[:first]), strings.TrimSpace(fact[first+len(Marker):]), count
}

// BoldSpan locates one **bold** span: Start/End cover the whole span including
// the asterisks; Text is the inner text.
type BoldSpan struct {
	Start, End int
	Text       string
}

// BoldSpans returns the **bold** spans in s, in order, ignoring code spans.
func BoldSpans(s string) []BoldSpan {
	var spans []BoldSpan
	for _, m := range boldRe.FindAllStringSubmatchIndex(maskCode(s), -1) {
		spans = append(spans, BoldSpan{Start: m[0], End: m[1], Text: s[m[2]:m[3]]})
	}
	return spans
}

// StripBold removes the ** markers of every bold span in s, keeping the text.
func StripBold(s string) string {
	return ReplaceBold(s, -1, "")
}

// ReplaceBold unwraps every bold span in s, except span number blank (0-based),
// which is replaced by with. Pass blank < 0 to unwrap all spans.
func ReplaceBold(s string, blank int, with string) string {
	var b strings.Builder
	prev := 0
	for i, sp := range BoldSpans(s) {
		b.WriteString(s[prev:sp.Start])
		if i == blank {
			b.WriteString(with)
		} else {
			b.WriteString(sp.Text)
		}
		prev = sp.End
	}
	b.WriteString(s[prev:])
	return b.String()
}

// IsQuestion reports whether a cue is an explicit question (ends with "?").
func IsQuestion(cue string) bool {
	return strings.HasSuffix(strings.TrimSpace(cue), "?")
}
