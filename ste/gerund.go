package ste

import "github.com/wow-look-at-my/slopfix/syntax"

// gerundDivision divides before an -ing form after a dash. The form says how
// the main verb acts, and the rest names a pronoun before the form as a
// finite verb: "benchmarks X -- using hyperfine" becomes "benchmarks X. It
// uses hyperfine". The pronoun and the verb agree with the main verb.
func gerundDivision(s *syntax.Sentence, c, main syntax.Clause, source string) (division, bool) {
	at := c.Link + 1
	if c.Kind != syntax.Punctuated || c.Depth != 0 || !dashMark(s.Words[c.Link].Text) || at+1 >= len(s.Words) ||
		s.Words[at].Tag != "VBG" || main.Verb == nil {
		return division{}, false
	}
	word := s.Words[at]
	// A gerund that is the subject of its own clause opens a sentence as it stands.
	if outsideSpans(source, word.Start, false) != word.Start || StandsAlone(capitalizeOpening(source[word.Start:])) {
		return division{}, false
	}
	pronoun, plural := "", false
	switch s.Words[main.Verb.Head].Tag {
	case "VBZ":
		pronoun = "It"
	case "VBP":
		pronoun, plural = "They", true
	default:
		return division{}, false
	}
	return division{
		leftEnd:    outsideSpans(source, s.Words[lastBefore(s, c.Link)].End, true),
		rightStart: s.Words[at+1].Start,
		opener:     pronoun + " " + presentOf(baseFromGerund(source[word.Start:word.End]), plural),
	}, true
}
