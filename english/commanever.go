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

// fixNeverVerb is the Fix text of a ", never" before a verb, where ", not" is no English.
const fixNeverVerb = `Write ", and never" before a verb. ` + "`slopfix fix` writes it."

// CheckCommaNever reports each ", never" in a prose block outside a code span or a quotation.
func CheckCommaNever(prose string, line int) []ste.Finding {
	var out []ste.Finding
	for _, at := range unmasked(prose) {
		fix := `Write ", not", or state the contrast as a sentence of its own. ` + "`slopfix fix` writes \", not\"."
		if beforeVerb(prose, at[1]) {
			fix = fixNeverVerb
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

// FixCommaNever writes ", not" for each ", never" that CheckCommaNever reports.
// Before a verb it writes ", and never": "it prompts, and never gates".
func FixCommaNever(prose string) string {
	var b strings.Builder
	last := 0
	for _, at := range unmasked(prose) {
		b.WriteString(prose[last:at[0]])
		to := ",${1}not"
		switch {
		case beforeVerb(prose, at[1]) && !finiteBefore(prose, at[0]) && !thirdPerson(prose[at[1]:]):
			to = ",${1}do not"
		case beforeVerb(prose, at[1]):
			to = ",${1}and never"
		}
		b.WriteString(commaNever.ReplaceAllString(prose[at[0]:at[1]], to))
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

// finiteBefore reports a finite verb in the sentence ahead of byte at. With
// none, the words ahead are an opening phrase, and "never" opens the instruction.
func finiteBefore(prose string, at int) bool {
	start := strings.LastIndex(prose[:at], ". ") + 1
	for _, w := range syntax.Parse(prose[start:at], nil).Words {
		switch w.Tag {
		case "VBZ", "VBP", "VBD", "MD":
			return true
		}
	}
	return false
}

// thirdPerson reports a verb in "-s" after an adverb run, which agrees with a subject and is no instruction: "never gates".
func thirdPerson(rest string) bool {
	for _, w := range strings.Fields(rest) {
		w = strings.ToLower(strings.Trim(w, ".,;:!?"))
		if strings.HasSuffix(w, "ly") {
			continue
		}
		return strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss")
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
