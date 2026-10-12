package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// rewriteWould writes each "would" verb group of prose in the present tense,
// read off the parse. "the build would fail" becomes "the build fails", "it
// would be faster" becomes "it is faster", "it would not run" becomes "it
// does not run", and "would it fail?" becomes "does it fail?". "Will" would
// turn a condition into a promise, so the repair never writes it.
func rewriteWould(prose string) string {
	s := syntax.Parse(checkMask(prose), nil)
	var out strings.Builder
	last := 0
	for i := 0; i < len(s.Words); i++ {
		w := s.Words[i]
		if w.Lower() != "would" || inName(prose, w.Start, w.End) {
			continue
		}
		end, text := wouldGroup(s, prose, i)
		if isCapitalized(prose[w.Start:w.End]) {
			text = capitalize(text)
		}
		out.WriteString(prose[last:w.Start])
		out.WriteString(text)
		last = end
	}
	out.WriteString(prose[last:])
	return out.String()
}

// wouldGroup answers where the verb group that opens with "would" at word i
// ends, and the present-tense words that replace it.
func wouldGroup(s *syntax.Sentence, prose string, i int) (int, string) {
	question := i == 0 || s.Words[i-1].Text == "," && i == 1
	plural := false
	j := i + 1
	if question {
		// "Would it fail": the subject comes between the modal and its verb.
		k := j
		for k < len(s.Words) && !strings.HasPrefix(s.Words[k].Tag, "VB") {
			k++
		}
		plural = pluralSubject(s, j, k)
		if k >= len(s.Words) {
			return s.Words[i].End, presentAux(plural, false)
		}
		base := verbBase(s, k)
		subject := prose[s.Words[i].End:s.Words[k].Start]
		if base == "be" {
			return s.Words[k].End, presentBe(plural) + strings.TrimRight(subject, " ")
		}
		return s.Words[i].End, presentAux(plural, false)
	}
	plural = pluralSubject(s, 0, i)
	negated := false
	var adverbs []string
	for j < len(s.Words) && (s.Words[j].Tag == "RB" || s.Words[j].Lower() == "not") {
		if s.Words[j].Lower() == "not" || s.Words[j].Lower() == "n't" {
			negated = true
		} else {
			adverbs = append(adverbs, s.Words[j].Text)
		}
		j++
	}
	// "would have failed" is the same condition, written about a past moment.
	if j+1 < len(s.Words) && s.Words[j].Lower() == "have" && s.Words[j+1].Tag == "VBN" {
		j++
	}
	if j >= len(s.Words) || !strings.HasPrefix(s.Words[j].Tag, "VB") {
		return s.Words[i].End, presentAux(plural, negated)
	}
	base := verbBase(s, j)
	lead := ""
	if len(adverbs) > 0 {
		lead = strings.Join(adverbs, " ") + " "
	}
	switch {
	case base == "be":
		text := presentBe(plural)
		if negated {
			text += " not"
		}
		if lead != "" {
			text += " " + strings.TrimSpace(lead)
		}
		return s.Words[j].End, text
	case negated:
		return s.Words[j].End, presentAux(plural, true) + " " + lead + base
	}
	return s.Words[j].End, lead + presentOf(base, plural)
}

// verbBase answers the base form of the verb at word k.
func verbBase(s *syntax.Sentence, k int) string {
	w := s.Words[k]
	lower := w.Lower()
	switch {
	case lower == "been" || lower == "be":
		return "be"
	case w.Tag == "VBN" || w.Tag == "VBD":
		if base := baseFromParticiple(lower); base != "" {
			return base
		}
	}
	return lower
}

// pluralSubject reports a plural subject among words from up to end.
func pluralSubject(s *syntax.Sentence, from, end int) bool {
	for k := end - 1; k >= from && k >= 0; k-- {
		w := s.Words[k]
		switch strings.ToLower(w.Text) {
		case "they", "we", "you", "i", "these", "those":
			return true
		case "it", "he", "she", "this", "that":
			return false
		}
		if w.Tag == "NNS" || w.Tag == "NNPS" {
			return true
		}
		if strings.HasPrefix(w.Tag, "NN") {
			return false
		}
	}
	return false
}

// presentBe answers "is" or "are".
func presentBe(plural bool) string {
	if plural {
		return "are"
	}
	return "is"
}

// presentAux answers the auxiliary a present-tense negative or question takes.
func presentAux(plural, negated bool) string {
	aux := "does"
	if plural {
		aux = "do"
	}
	if negated {
		return aux + " not"
	}
	return aux
}
