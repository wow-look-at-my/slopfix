package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// restatedOpening answers the first prose paragraph of text as one restated
// sentence that carries a later "that" clause with the verb. That governs
// it, laid out under a leading heading where both fit. It reports false when
// the whole paragraph fits, or when no restatement does.
func restatedOpening(text []string, fit layout) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return nil, false
	}
	for _, para := range paragraphs(text) {
		if para.blank || para.verbatim {
			continue
		}
		joined := closeThoughts(para.lines)
		if !endsSentence(joined) {
			joined += "."
		}
		heads := [][]string{nil}
		if head := sectionHead(text); head != nil {
			heads = [][]string{head, nil}
		}
		sentences := ste.Sentences(ste.FixKeepingOpening(joined))
		for _, head := range heads {
			if _, wholeFits := fit(head, joined, indent, marker); wholeFits {
				return nil, false
			}
			for _, restated := range sharedHeadRestated(sentences) {
				if out, ok := fit(head, restated, indent, marker); ok {
					return out, true
				}
			}
		}
		return nil, false
	}
	return nil, false
}

// sharedHeadRestated answers the restatements of a paragraph's first
// sentence that keep its verb and carry a later "that" clause. The clause
// either sits in the first sentence behind ", and that", or opens a sentence
// that restates the verb behind "also": "The caller must also ensure that B."
// gives "The caller must ensure that B." The longest comes first.
func sharedHeadRestated(sentences []string) []string {
	if len(sentences) == 0 {
		return nil
	}
	first := strings.TrimSpace(sentences[0])
	out := sharedHeadTails(first)
	at := strings.Index(first, " that ")
	if at < 0 {
		return out
	}
	lead := first[:at+len(" that ")]
	for _, later := range sentences[1:] {
		restated := strings.Replace(strings.TrimSpace(later), " also ", " ", 1)
		if strings.HasPrefix(restated, lead) && ste.StandsAlone(restated) {
			out = append(out, restated)
		}
	}
	return out
}

// sharedHeadTails answers the sentences that keep the verb in front of the
// first "that" and one later ", and that" clause. "The caller must ensure that
// A, and that B." gives "The caller must ensure that B." The longest tail comes
// first, and only a tail that stands alone as a sentence is answered.
func sharedHeadTails(sentence string) []string {
	const joint = ", and that "
	first := strings.Index(sentence, " that ")
	if first < 0 {
		return nil
	}
	lead := sentence[:first+len(" that ")]
	var out []string
	for from := first + len(" that "); ; {
		at := strings.Index(sentence[from:], joint)
		if at < 0 {
			return out
		}
		from += at + len(joint)
		if tail := lead + sentence[from:]; ste.StandsAlone(tail) {
			out = append(out, tail)
		}
	}
}
