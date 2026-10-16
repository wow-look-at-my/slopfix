// open.go reads the words after a cut as a sentence of their own, and says how
// well that rest reads.

package ste

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// How well each way of opening the rest reads, added to a cut's score.
const (
	opensOwnClause = 6
	opensWithVerb  = 3
)

// openRest writes the words after a cut as a sentence of their own, and says
// how well it reads. A clause that names its own subject opens as it is, and
// so does an imperative after an imperative. A finite verb after a conjunction
// gets the subject again. Any other rest is no sentence, and openRest answers "".
func openRest(source, masked string, whole *syntax.Sentence, c forceCut) (string, int) {
	rest := strings.TrimLeft(source[c.right:], " —–-,;:")
	restMasked := masked[len(masked)-len(rest):]
	if rest == "" {
		return "", 0
	}
	seam := seamBefore(source, c.left)
	opener, conjunction := "", ""
	if word := strings.ToLower(firstToken.FindString(rest)); word != "" {
		if connector, ok := connectors[word]; ok {
			cut := len(firstToken.FindString(rest))
			trimmed := strings.TrimLeft(rest[cut:], " ")
			if trimmed == "" {
				return "", 0
			}
			rest = strings.TrimLeft(trimmed, " —–-,;:")
			if rest == "" {
				return "", 0
			}
			restMasked = restMasked[len(restMasked)-len(rest):]
			opener, conjunction = connector, word
		}
	}
	// A cut between words with no mark and no conjunction lands inside a clause.
	if seam == "" && conjunction == "" {
		return "", 0
	}
	// A capital renames an identifier written in lower case, and Check reads no sentence start there.
	if lowerIdentifier(firstToken.FindString(rest)) {
		return "", 0
	}
	s := syntax.Parse(opening(restMasked), nil)
	first := wordFrom(whole, len(source)-len(rest))
	if len(s.Words) == 0 || first < 0 {
		return "", 0
	}
	strong := seam == "—" || seam == "–" || seam == "--" || seam == ":"
	switch tag := whole.Words[first].Tag; {
	case strong && conjunction != "so" && !lowerIdentifier(firstToken.FindString(rest)) && opensImperative(restMasked):
		// A dash or a colon before an instruction ends the sentence before it: "— but be economical".
		return joinOpener(opener, rest), opensOwnClause - 1
	case conjunction == "so" && seam == "," && !opensImperativeMain(masked[:c.left]) && !instructs(whole) && opensImperative(restMasked):
		// A statement, then ", so" and an instruction: the instruction follows from the statement, as a sentence of its own.
		return joinOpener(opener, rest), opensOwnClause
	case (tag == "VB" || tag == "VBP") && opensImperativeMain(masked[:c.left]) && opensImperative(restMasked):
		// An imperative joins only another imperative, and never after a bare comma, where it is an item of a list.
		if seam == "," && conjunction == "" || conjunction == "so" {
			return "", 0
		}
		return joinOpener(opener, rest), opensOwnClause
	case tag == "VBZ" || tag == "VBP" || tag == "VBD" || tag == "MD":
		// Verb groups after a bare comma are a list, and a list never divides.
		if conjunction == "" || conjunction == "so" || listsVerbs(masked[:c.left]) {
			return "", 0
		}
		subject := subjectFor(source, masked, c, tag)
		if subject == "" {
			return "", 0
		}
		return joinOpener(opener, subject+" "+rest), opensWithVerb
	// A verb that only a relative clause holds leaves the rest a noun phrase: "a subordinator that opens a clause".
	case opensClause(s) && opensSubject(whole, first) && agrees(s, *s.Clauses[0].Subject, *s.Clauses[0].Verb) &&
		verbOfSubject(s, *s.Clauses[0].Subject, *s.Clauses[0].Verb) && finiteOutsideRelative(s, 0):
		// A so with no comma before it, or after an instruction, states a purpose.
		if conjunction == "so" && (seam != "," || opensImperativeMain(masked[:c.left]) || instructs(whole)) {
			return "", 0
		}
		// After a list, ", and" adds the last item to it: "rules, hooks, and `ask` rules apply".
		if seam == "," && listsVerbs(masked[:c.left]) {
			return "", 0
		}
		return joinOpener(opener, rest), opensOwnClause
	case (conjunction == "and" || conjunction == "so") && (seam == "," || seam == "—" || seam == "–" || seam == "--") &&
		opensNounPhrase(tag) && !lowerIdentifier(firstToken.FindString(rest)) && StandsAlone(capitalizeOpening(rest)):
		// The parse cuts its clauses at commas, so a subject that is a list or
		// carries a participle opens no clause of its own.
		if conjunction == "so" && (opensImperativeMain(masked[:c.left]) || instructs(whole)) || listsVerbs(masked[:c.left]) ||
			conjunction == "and" && seriesBefore(masked[:c.left]) {
			return "", 0
		}
		return joinOpener(opener, rest), opensOwnClause - 1
	case (conjunction == "or" || conjunction == "but") && opensNounPhrase(tag) && !listsVerbs(masked[:c.left]) &&
		!lowerIdentifier(firstToken.FindString(rest)) && StandsAlone(capitalizeOpening(rest)):
		// "MUST agree or growth would ping-pong": a clause after "or" with its own subject is the alternative, behind "Otherwise,".
		return joinOpener(opener, rest), opensOwnClause - 1
	case conjunction == "" && (seam == "—" || seam == "–" || seam == ":" || seam == "--") && !lowerIdentifier(firstToken.FindString(rest)):
		// A dash or a colon before words that hold a main clause of their own: "— on first launch Grok runs your binary".
		out := capitalizeOpening(rest)
		if StandsAlone(out) {
			return out, opensOwnClause - 1
		}
		if out, ok := commaAfterFrontedPhrase(out); ok && StandsAlone(out) {
			return out, opensOwnClause - 1
		}
	}
	return "", 0
}

// opensNounPhrase reports a tag that starts a subject.
func opensNounPhrase(tag string) bool {
	return tag == "DT" || tag == "PRP" || tag == "PRP$" || tag == "JJ" || tag == "CD" || strings.HasPrefix(tag, "NN")
}

// seriesBefore reports a comma after the first verb of head. A ", and" after it adds the last item of a series: "give the full path, why it matters, and the relevant code".
func seriesBefore(head string) bool {
	s := syntax.Parse(strings.TrimRight(head, " ,"), nil)
	for i, w := range s.Words {
		if !strings.HasPrefix(w.Tag, "VB") {
			continue
		}
		for _, later := range s.Words[i+1:] {
			if later.Text == "," {
				return true
			}
		}
		return false
	}
	return false
}

// commaAfterFrontedPhrase writes the comma that ends a participle phrase in front of its clause: "Left in the text a rewrite wraps it" becomes "Left in the text, a rewrite wraps it".
func commaAfterFrontedPhrase(text string) (string, bool) {
	s := syntax.Parse(opening(text), nil)
	if len(s.Words) < 4 || s.Words[0].Tag != "VBN" {
		return text, false
	}
	for i := 2; i < len(s.Words); i++ {
		w := s.Words[i]
		if w.Text == "," {
			return text, false
		}
		if (w.Tag == "DT" || w.Tag == "PRP$") && strings.HasPrefix(s.Words[i-1].Tag, "NN") {
			at := s.Words[i-1].End
			return text[:at] + "," + text[at:], true
		}
	}
	return text, false
}

// openingBytes bounds how much of the rest the parser reads. Only its first clause decides how the rest opens.
const openingBytes = 240

// opening answers the start of text, cut back to a blank.
func opening(text string) string {
	if len(text) <= openingBytes {
		return text
	}
	if at := strings.LastIndexByte(text[:openingBytes], ' '); at > 0 {
		return text[:at]
	}
	return text
}

// subjectFor names the subject of the words before the cut again, for a verb
// that opens the rest. A short subject repeats. A long one becomes a pronoun
// that agrees with the verb. With no subject to name, it answers "".
func subjectFor(source, masked string, c forceCut, tag string) string {
	s := syntax.Parse(masked[:c.left], nil)
	// Only the clause right before the cut shares its subject with the verb after it.
	for _, clause := range s.Clauses[max(len(s.Clauses)-1, 0):] {
		if clause.Depth != 0 || clause.Subject == nil || clause.Verb == nil {
			continue
		}
		subject := *clause.Subject
		if subject.Last >= clause.Verb.First || !opensSubject(s, subject.First) {
			return ""
		}
		// A later finite verb has a subject of its own, which the parse did not name.
		for _, w := range s.Words[clause.Verb.Last+1:] {
			if w.Tag == "VBZ" || w.Tag == "VBP" || w.Tag == "VBD" || w.Tag == "MD" {
				return ""
			}
		}
		head := s.Words[subject.Head]
		if head.Tag == "PRP" {
			return lowerOpening(head.Text)
		}
		if subject.Last-subject.First < restateLimit && !subject.Coordinated {
			text := source[s.Words[subject.First].Start:s.Words[subject.Last].End]
			if first := s.Words[subject.First]; first.Tag == "DT" || first.Tag == "PRP$" {
				text = lowerOpening(text)
				if det := strings.ToLower(first.Text); det == "a" || det == "an" {
					text = "the" + text[len(det):]
				}
			}
			return text
		}
		if tag == "VBP" || tag != "VBZ" && s.Plural(subject) {
			return "they"
		}
		if s.Person(subject) {
			return ""
		}
		return "it"
	}
	return ""
}

// lowerOpening writes the first letter in lower case.
func lowerOpening(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(first)) + s[width:]
}

// lastField answers the last blank-separated word of s.
func lastField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}
