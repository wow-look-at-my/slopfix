// split_clauses.go answers where a clause starts, resumes and closes, and restates a clause as a sentence.

package ste

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// finiteBetween reports a finite verb among words from up to end.
func finiteBetween(s *syntax.Sentence, from, end int) bool {
	for i := max(from, 0); i < end && i < len(s.Words); i++ {
		if finiteAt(s, i) {
			return true
		}
	}
	return false
}

// resumesAfter reports whether a main clause with a subject starting at word i
// resumes after a relative clause, as in ": a map that carries nothing is wasteful".
func resumesAfter(s *syntax.Sentence, i int) bool {
	for _, c := range s.Clauses {
		if c.Kind == syntax.Opens && c.Depth == 0 && c.Verb != nil && c.Subject != nil && c.Subject.First == i {
			return true
		}
	}
	return false
}

// closesTheSentence reports whether the clause and the clauses under it run to
// the end of the sentence. A clause the sentence returns from, as in "the file,
// which fails, is gone", cannot stand alone.
func closesTheSentence(s *syntax.Sentence, c syntax.Clause) bool {
	after := false
	for _, later := range s.Clauses {
		if later.First == c.First {
			after = true
			continue
		}
		if after && later.Depth < c.Depth && later.Kind != syntax.Coordinate {
			return false
		}
	}
	for i := c.Link + 1; i+1 < len(s.Words); i++ {
		if s.Words[i].Text == "," && strings.HasPrefix(s.Words[i+1].Tag, "VB") {
			return false
		}
	}
	return true
}

// opensWithCapital reports whether the word at i reads right with a capital.
// A name written in lower case, such as a command, does not. A code span does
// not either, whatever word the mask wrote over it.
func opensWithCapital(s *syntax.Sentence, i int, source string) bool {
	w := s.Words[i]
	if outsideSpans(source, w.Start+1, false) != w.Start+1 {
		return false
	}
	first, _ := utf8.DecodeRuneInString(w.Text)
	return !(unicode.IsLower(first) && (w.Tag == "NNP" || w.Tag == "NNPS"))
}

// restated names the main clause's subject again, for the verb group of c that
// shared it. A short subject repeats, and an indefinite article becomes the.
// Otherwise a pronoun stands in, chosen to agree with c's verb. It answers false
// when no pronoun agrees, as for a single person and a verb in -s.
func restated(s *syntax.Sentence, main, c syntax.Clause, source string) (string, bool) {
	if main.Subject == nil || main.Verb == nil || c.Verb == nil {
		return "", false
	}
	// A subject before an -ing form writes "It using hyperfine", which is no sentence.
	if s.Words[c.Verb.First].Tag == "VBG" || c.Link >= 0 && c.Link+1 < len(s.Words) && s.Words[c.Link+1].Tag == "VBG" {
		return "", false
	}
	subject := *main.Subject
	if subject.First > 0 && s.Words[subject.First-1].Tag == "VBG" {
		// The noun phrase is the object of a gerund, and the gerund is the subject: "freezing the HOW pins it".
		return "", false
	}
	// A subject the parser found after its verb, or one that opens on a bare noun, is a misreading.
	if subject.Last >= main.Verb.First || !opensSubject(s, subject.First) {
		return "", false
	}
	head := s.Words[subject.Head]
	if head.Tag == "PRP" {
		return head.Text, true
	}
	short := restates(s, subject) && s.Words[subject.Last+1].Tag != "IN"
	if short && opensWithCapital(s, subject.First, source) {
		// A noun that disagrees with the verb is not that verb's subject: "its reads ... keeps".
		if !agrees(s, subject, *c.Verb) {
			return "", false
		}
		text := source[s.Words[subject.First].Start:s.Words[subject.Last].End]
		if subject.Det == subject.First {
			if det := s.Words[subject.Det].Lower(); det == "a" || det == "an" {
				text = "the" + text[len(det):]
			}
		}
		return text, true
	}
	plural := s.Plural(subject)
	switch s.Words[c.Verb.Head].Tag {
	case "VBZ":
		plural = false
	case "VBP":
		plural = true
	}
	switch {
	case plural:
		return "they", true
	case s.Person(subject):
		return "", false
	}
	return "it", true
}

// spliced reports whether a comma the splice pattern matched joins main clauses.
func spliced(prose string, loc []int) bool {
	if loc[2] >= 0 {
		return joinsClauses(prose, loc[2])
	}
	return isClause(clauseBefore(prose, loc[0])) && endsMainClause(prose, loc[0])
}

// endsMainClause reports whether a main clause runs up to the comma at byte
// at. "If the cache is cold," and "where each run gets a machine," do not.
func endsMainClause(prose string, at int) bool {
	for _, span := range sentenceSpans(prose) {
		if at < span[0] || at >= span[1] {
			continue
		}
		text := prose[span[0]:span[1]]
		s := syntax.Parse(text, opaque(text, text))
		last := -1
		for i, w := range s.Words {
			if w.End <= at-span[0] {
				last = i
			}
		}
		for _, c := range s.Clauses {
			if c.First <= last && last <= c.Last {
				return c.Depth == 0 && c.Verb != nil && standsAlone(s, 0, last+1)
			}
		}
		return false
	}
	return false
}

// joinsClauses reports whether the conjunction at byte at joins a main clause
// to another that names its own subject. The end of a list does not.
func joinsClauses(prose string, at int) bool {
	word, _, _ := strings.Cut(prose[at:], " ")
	if !syntax.Is(word, "coordinator") {
		// "then" is an adverb to the parser, and the splice pattern alone judges it.
		return true
	}
	for _, span := range sentenceSpans(prose) {
		if at < span[0] || at >= span[1] {
			continue
		}
		s := syntax.Parse(prose[span[0]:span[1]], opaque(prose[span[0]:span[1]], prose[span[0]:span[1]]))
		for k, c := range s.Clauses {
			if c.Link >= 0 && s.Words[c.Link].Start == at-span[0] {
				if c.Kind != syntax.Coordinate || c.Depth != 0 || c.Subject == nil || listBefore(s, c) {
					return false
				}
				// Each half must be a sentence: a main clause before, and a subject that
				// agrees with its verb after.
				if k == 0 || s.Clauses[k-1].Verb == nil || !standsAlone(s, 0, c.Link) || !subjectFollows(s, c) || !agrees(s, *c.Subject, *c.Verb) {
					return false
				}
				// A so after an instruction states its purpose, and joins nothing a period can take.
				main, ok := mainBefore(s, k)
				return !strings.EqualFold(word, "so") || ok && !main.Verb.Imperative && !instructs(s)
			}
		}
		return false
	}
	return false
}

// listBefore reports a comma inside the clause before c, which makes ", and"
// the end of a list rather than a join between clauses.
func listBefore(s *syntax.Sentence, c syntax.Clause) bool {
	for i := c.Link - 2; i >= 0 && i >= clauseStart(s, c.Link); i-- {
		// A comma inside a quotation reads as a name, and lists nothing.
		if s.Words[i].Text == "," && s.Words[i].Tag == "," {
			return true
		}
	}
	return false
}

// clauseStart answers the earliest word of the clause that ends before word i.
func clauseStart(s *syntax.Sentence, i int) int {
	start := 0
	for _, c := range s.Clauses {
		if c.First < i && c.First > start && c.Link != i {
			start = c.First
		}
	}
	return start
}

const restateLimit = 4
