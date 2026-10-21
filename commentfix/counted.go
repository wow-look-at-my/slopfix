package commentfix

import (
	"regexp"
	"slices"
	"strings"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// governed is a number and the word after it, which is the noun it counts.
var governed = regexp.MustCompile(`^\S+\s+[A-Za-z][A-Za-z-]*`)

// compoundWords bounds the words from a number to the plural noun it counts, as in "0 validation errors".
const compoundWords = 3

// countedPhrase answers the number at offset at and the words up to the plural
// noun it counts, as in "0 validation errors". It answers nothing when no
// plural noun ends a phrase the number governs.
func countedPhrase(prose string, at int) string {
	words := syntax.Parse(prose, nil).Words
	k := slices.IndexFunc(words, func(w syntax.Word) bool { return w.Start == at })
	if k < 0 {
		// The tokenizer joined the number to a mark, so the next word is the noun it counts, or none.
		phrase := governed.FindString(prose[at:])
		if phrase == "" {
			return ""
		}
		zeroSubject := strings.HasPrefix(phrase, "0 ") && cardinal.ValueSubject(prose, cardinal.Match{At: at, Text: phrase})
		if !zeroSubject && cardinal.NotAPluralNoun(prose, cardinal.Match{At: at, Text: phrase}) {
			return ""
		}
		return phrase
	}
	if head := cardinal.CountedHead(words, k); head >= 0 && head <= k+compoundWords {
		return prose[at:words[head].End]
	}
	for j := k + 1; j < len(words) && j <= k+compoundWords; j++ {
		phrase := prose[at:words[j].End]
		if strings.HasPrefix(phrase, "0 ") && cardinal.ValueSubject(prose, cardinal.Match{At: at, Text: phrase}) {
			return phrase
		}
		if !cardinal.NotAPluralNoun(prose, cardinal.Match{At: at, Text: phrase}) {
			return phrase
		}
		// Only a modifier stands between a number and the noun it counts.
		if tag := words[j].Tag; tag != "NN" && tag != "NNP" && tag != "JJ" {
			return ""
		}
	}
	return ""
}

// rewordCounts takes each number out of the prose with counts.RewordAt, from
// the back, so an earlier offset stays valid.
func rewordCounts(prose string) string {
	tokens := cardinal.Find(prose, cardinal.Comment)
	for i := len(tokens) - 1; i >= 0; i-- {
		t := tokens[i]
		phrase := countedPhrase(prose, t.Offset)
		if phrase == "" {
			continue
		}
		if e, ok := counts.RewordAt(prose, t.Offset, phrase); ok {
			prose = prose[:e.Start] + e.Text + prose[e.End:]
		}
	}
	return prose
}
