package ste

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// IDPostdeterminer names the rule that cuts a numeral from "the three rules".
const IDPostdeterminer = "ste/postdeterminer"

// A postdeterminer is a numeral between a determiner and its noun. It restates
// a count the reader sees, and it goes stale when the set changes.
type postdeterminer struct {
	// Start and End are the numeral's bytes, and the space after it.
	Start, End int
	// Phrase is the noun phrase that carries it.
	Phrase string
}

// postdeterminers answers every redundant numeral in the prose.
func postdeterminers(prose string, opaque [][]int) []postdeterminer {
	var out []postdeterminer
	at := 0
	for _, sentence := range Sentences(prose) {
		start := strings.Index(prose[at:], sentence)
		if start < 0 {
			break
		}
		start += at
		at = start + len(sentence)
		s := syntax.Parse(sentence, shift(opaque, -start))
		for _, np := range s.NounPhrases() {
			if !redundantNumeral(s, np) {
				continue
			}
			first, last := s.Words[np.Numerals[0]], s.Words[np.Numerals[len(np.Numerals)-1]]
			end := last.End
			for end < len(sentence) && sentence[end] == ' ' {
				end++
			}
			out = append(out, postdeterminer{
				Start:  start + first.Start,
				End:    start + end,
				Phrase: s.Span(np.First, np.Last),
			})
		}
	}
	return out
}

// shift moves every span by delta.
func shift(spans [][]int, delta int) [][]int {
	out := make([][]int, len(spans))
	for i, span := range spans {
		out[i] = []int{span[0] + delta, span[1] + delta}
	}
	return out
}

// redundantNumeral reports whether a noun phrase carries a numeral a reader
// loses nothing without: a definite determiner, then the number, then a noun
// in the number the numeral agrees with.
func redundantNumeral(s *syntax.Sentence, np syntax.Phrase) bool {
	if len(np.Numerals) == 0 || np.Det < 0 {
		return false
	}
	det := s.Words[np.Det]
	if det.Tag != "POS" && !syntax.Is(det.Text, "definite") {
		return false
	}
	head := s.Words[np.Head]
	if head.Tag != "NN" && head.Tag != "NNS" {
		return false
	}
	for _, i := range np.Numerals {
		if !spelledCount(s.Words[i].Text) {
			return false
		}
	}
	after := np.Numerals[len(np.Numerals)-1] + 1
	if cardinal.IsUnit(s.Words[after].Text) || s.Words[after].Text == "%" {
		return false
	}
	single := len(np.Numerals) == 1 && strings.EqualFold(s.Words[np.Numerals[0]].Text, "one")
	return single == (head.Tag == "NN")
}

// spelledCount reports whether a numeral is a plain count: a whole number in
// digits, or a number word. A version, a decimal and a time are not.
func spelledCount(numeral string) bool {
	for _, r := range numeral {
		if !unicode.IsDigit(r) {
			return syntax.Is(numeral, "number-word")
		}
	}
	return len(numeral) < 4
}

func checkPostdeterminers(prose string, line int) []Finding {
	var out []Finding
	for _, hit := range postdeterminers(prose, nil) {
		out = append(out, Finding{
			Line:   line,
			ID:     IDPostdeterminer,
			Rule:   "a numeral between a determiner and its noun restates a count the reader can see",
			Detail: hit.Phrase,
			Fix:    "Cut the numeral.",
		})
	}
	return out
}

// fixPostdeterminers cuts every redundant numeral from the prose.
func fixPostdeterminers(prose string) string {
	masked := mask(prose)
	hits := postdeterminers(masked, offLimits(prose, masked))
	for i := len(hits) - 1; i >= 0; i-- {
		prose = prose[:hits[i].Start] + prose[hits[i].End:]
	}
	return prose
}
