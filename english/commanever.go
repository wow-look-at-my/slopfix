package english

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// IDCommaNever names the rule against a contrast added with ", never".
const IDCommaNever = "english/comma-never"

// AllIDs names every rule the english category reports.
var AllIDs = []string{IDCommaNever}

// commaNever is rare in edited English, which writes ", not" for the same contrast.
var commaNever = regexp.MustCompile(`(?i),(\s+)never\b`)

// FixNeverByHand is the Fix text of a ", never" before a verb, where ", not" is no English.
const FixNeverByHand = `Rewrite it by hand. A verb follows "never", and ", not" before a verb leaves no sentence.`

// CheckCommaNever reports each ", never" in a prose block outside a code span or a quotation.
func CheckCommaNever(prose string, line int) []ste.Finding {
	var out []ste.Finding
	for _, at := range unmasked(prose) {
		fix := `Write ", not", or state the contrast as a sentence of its own. ` + "`slopfix fix` writes \", not\"."
		if beforeVerb(prose, at[1]) {
			fix = FixNeverByHand
		}
		out = append(out, ste.Finding{
			Line:   line,
			ID:     IDCommaNever,
			Rule:   "a contrast added as an afterthought",
			Detail: prose[at[0]:at[1]],
			Fix:    fix,
		})
	}
	return out
}

// FixCommaNever writes ", not" for each ", never" that CheckCommaNever reports,
// except before a verb: "it prompts, never gates" has no repair with "not".
func FixCommaNever(prose string) string {
	var b strings.Builder
	last := 0
	for _, at := range unmasked(prose) {
		if beforeVerb(prose, at[1]) {
			continue
		}
		b.WriteString(prose[last:at[0]])
		b.WriteString(commaNever.ReplaceAllString(prose[at[0]:at[1]], ",${1}not"))
		last = at[1]
	}
	b.WriteString(prose[last:])
	return b.String()
}

// beforeVerb reports whether the word after byte at, the end of a ", never", is
// a bare or finite verb, read in the sentence around it. A participle reads
// right after "not", as in ", not stacked".
func beforeVerb(prose string, at int) bool {
	start := strings.LastIndex(prose[:at], ". ") + 1
	end := len(prose)
	if next := strings.Index(prose[at:], ". "); next >= 0 {
		end = at + next + 1
	}
	s := syntax.Parse(prose[start:end], nil)
	for i, w := range s.Words {
		// An adverb between them leaves the verb after "never": "never silently allow".
		if w.Start < at-start || w.Tag == "RB" {
			continue
		}
		switch w.Tag {
		case "VB", "VBP", "VBZ", "VBD", "MD":
			return true
		}
		// A word with an object pronoun after it is a verb the tagger missed: "never auto-allow it".
		return i+1 < len(s.Words) && objects.Contains(s.Words[i+1].Lower())
	}
	return false
}

// objects are the pronouns that only an object takes.
var objects = set.Of("it", "them", "him", "her", "us", "me")

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
