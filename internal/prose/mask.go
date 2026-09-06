package prose

import (
	"regexp"
	"strings"
)

// The maskers below replace a span with the same number of X runes. Positions
// therefore survive masking, so a finding can still quote the original text.
var (
	entityRe   = regexp.MustCompile(`&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[a-zA-Z][a-zA-Z0-9]{1,31});`)
	autolinkRe = regexp.MustCompile(`<[a-zA-Z][a-zA-Z0-9+.-]*:[^>\s]*>`)
	urlRe      = regexp.MustCompile(`\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s)>\]]+`)
	linkDestRe = regexp.MustCompile(`\]\([^)\s]*(?:\s+"[^"]*")?\)`)
	codeSpanRe = regexp.MustCompile("(`+)(?:[^`]|(?:`(?!`)))*?(`+)")
)

// Mask blanks out every span a prose rule must not read, and returns a string
// of the same length as the input.
//
// An HTML entity is masked because it ENDS IN A SEMICOLON. Left alone,
// `&lpar;` reports a banned semicolon in a sentence that has none. That false
// positive is the reason this function exists.
func Mask(s string) string {
	out := s
	for _, re := range []*regexp.Regexp{codeSpanRe, autolinkRe, urlRe, linkDestRe, entityRe} {
		out = re.ReplaceAllStringFunc(out, blank)
	}
	return out
}

func blank(s string) string {
	return strings.Repeat("X", len([]rune(s)))
}
