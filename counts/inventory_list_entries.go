// inventory_list_entries.go judges whether a table row or a list item only names things.

package counts

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
)

var (
	entryJoiners = set.Of("and", "or", "nor", "plus", "&")
	// shortLabel matches a short parenthesis: a label beside the members of an entry.
	shortLabel = regexp.MustCompile(`\(\s*[^()\s]+(?:\s+[^()\s]+){0,2}\s*\)`)
)

// NamesOnly answers the number of members text names when it names members
// and says nothing about them: members, joiners, punctuation and one short
// parenthetical label. It answers zero for text that states or describes
// something.
func NamesOnly(text string) int {
	masked := maskSpans(text)
	labels := 0
	for _, loc := range shortLabel.FindAllStringIndex(masked, -1) {
		if holdsMember(text[loc[0]:loc[1]]) {
			continue
		}
		labels++
		if labels > 1 {
			return 0
		}
		text = text[:loc[0]] + strings.Repeat(" ", loc[1]-loc[0]) + text[loc[1]:]
	}
	members := 0
	for _, tok := range tokenPattern.FindAllString(text, -1) {
		bare := strings.Trim(tok, "*_~")
		switch {
		case IsMember(bare) || IsMember(strings.TrimRight(strings.TrimLeft(bare, "("), ").,;:!?")):
			members++
		case strings.Trim(bare, ".,;:!?()[]-–—/|") == "":
		case entryJoiners.Contains(strings.ToLower(strings.Trim(bare, ".,;:!?"))):
		default:
			return 0
		}
	}
	return members
}

// maskSpans hides what each code span holds, so a parenthesis inside one is not a label.
func maskSpans(text string) string {
	out := []byte(text)
	for _, loc := range inlineCode.FindAllStringIndex(text, -1) {
		for i := loc[0]; i < loc[1]; i++ {
			out[i] = '`'
		}
	}
	return string(out)
}

// keepsWords reports whether prose, with each span taken out, still holds a
// word that is not a member. A cell, a heading or an item takes such a cut
// only then.
func keepsWords(prose string, spans []Span) bool {
	rest := CutProse(prose, spans)
	if NamesOnly(rest) > 0 {
		return false
	}
	for _, tok := range tokenPattern.FindAllString(rest, -1) {
		if !IsMember(strings.Trim(tok, "*_~")) && strings.IndexFunc(tok, unicode.IsLetter) >= 0 {
			return true
		}
	}
	return false
}

// spansOf answers the spans of cuts.
func spansOf(cuts []ProseCut) []Span {
	out := make([]Span, len(cuts))
	for i, c := range cuts {
		out[i] = c.Span
	}
	return out
}
