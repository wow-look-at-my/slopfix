package english

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// IDCommaNever names the rule against a contrast added with ", never".
const IDCommaNever = "english/comma-never"

// AllIDs names every rule the english category reports.
var AllIDs = []string{IDCommaNever}

// commaNever is rare in edited English, which writes ", not" for the same contrast.
var commaNever = regexp.MustCompile(`(?i),(\s+)never\b`)

// CheckCommaNever reports each ", never" in a prose block outside a code span or a quotation.
func CheckCommaNever(prose string, line int) []ste.Finding {
	var out []ste.Finding
	for _, at := range unmasked(prose) {
		out = append(out, ste.Finding{
			Line:   line,
			ID:     IDCommaNever,
			Rule:   `a contrast added with ", never"`,
			Detail: prose[at[0]:at[1]],
			Fix:    `Write ", not", or state the contrast as a sentence of its own. ` + "`slopfix fix` writes \", not\".",
		})
	}
	return out
}

// FixCommaNever writes ", not" for each ", never" that CheckCommaNever reports.
func FixCommaNever(prose string) string {
	var b strings.Builder
	last := 0
	for _, at := range unmasked(prose) {
		b.WriteString(prose[last:at[0]])
		b.WriteString(commaNever.ReplaceAllString(prose[at[0]:at[1]], ",${1}not"))
		last = at[1]
	}
	b.WriteString(prose[last:])
	return b.String()
}

// unmasked answers each match that no code span or quotation holds, because their words are data.
func unmasked(prose string) [][]int {
	masks := append(codeSpan.FindAllStringIndex(prose, -1), quotation.FindAllStringIndex(prose, -1)...)
	var out [][]int
	for _, at := range commaNever.FindAllStringIndex(prose, -1) {
		if !insideAny(masks, at[0], at[1]) {
			out = append(out, at)
		}
	}
	return out
}
