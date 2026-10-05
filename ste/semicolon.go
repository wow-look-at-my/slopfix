package ste

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// FixSemicolonByHand is the Fix text of a semicolon that no period can replace.
const FixSemicolonByHand = "Rewrite it by hand. The semicolon joins items or phrases, and a period leaves a fragment on one side."

// ByHand reports whether a Fix text asks for a rewrite by hand, which no repair writes.
func ByHand(fix string) bool { return strings.HasPrefix(fix, "Rewrite it by hand") }

// semicolonJoins answers the offset of each semicolon of masked that a period
// can replace. A semicolon divides only where the words before it hold a main
// clause and the words after it open one.
func semicolonJoins(masked string) []int {
	var joins []int
	for _, span := range sentenceSpans(masked) {
		text := masked[span[0]:span[1]]
		if !strings.Contains(text, ";") {
			continue
		}
		s := syntax.Parse(text, nil)
		from, colon := 0, -1
		var found []int
		listed := true
		for i, w := range s.Words {
			if w.Text == ":" && colon < 0 {
				colon = i
			}
			if w.Text != ";" {
				continue
			}
			// The words since the last colon or semicolon are what a period closes,
			// and a list item there is no sentence: "it runs: building, testing.
			if !semicolonDivides(s, max(from, segmentStart(s, i)), i) {
				listed = listed && colon < 0
				continue
			}
			found = append(found, span[0]+w.Start)
			from = i + 1
		}
		// The semicolons of a list after a colon divide all items or none.
		if colon >= 0 && !listed {
			found = beforeWord(found, span[0]+s.Words[colon].Start)
		}
		joins = append(joins, found...)
	}
	return joins
}

// beforeWord keeps the offsets less than at.
func beforeWord(offsets []int, at int) []int {
	var kept []int
	for _, o := range offsets {
		if o < at {
			kept = append(kept, o)
		}
	}
	return kept
}

// segmentStart answers the first word after the last colon or semicolon ahead of word i.
func segmentStart(s *syntax.Sentence, i int) int {
	for j := i - 1; j >= 0; j-- {
		if t := s.Words[j].Text; t == ";" || t == ":" {
			return j + 1
		}
	}
	return 0
}

// semicolonDivides reports whether the semicolon at word i of s ends. A
// sentence that opens at word from, and the words after it open a sentence of
// their own. A semicolon that ends the sentence always divides.
func semicolonDivides(s *syntax.Sentence, from, i int) bool {
	rest := i + 1
	for rest < len(s.Words) && strings.IndexFunc(s.Words[rest].Text, unicode.IsLetter) < 0 {
		rest++
	}
	if rest == len(s.Words) {
		return true
	}
	return standsAlone(s, from, i) && opensSentenceAt(s, i)
}

// opensSentenceAt reports whether the words. This happens after the mark at
// word i open with their own subject and a verb that agrees with it, with an
// imperative, or with a subordinate clause and then its main clause.
func opensSentenceAt(s *syntax.Sentence, i int) bool {
	for k, c := range s.Clauses {
		if c.Depth == 0 && c.Link == i {
			if c.Subject != nil {
				return subjectFollows(s, c) && agrees(s, *c.Subject, *c.Verb)
			}
			if resumesAfter(s, i+1) && opensSubject(s, i+1) {
				return true
			}
			// The tagger reads a bare verb after the mark as a noun, so the words are read with the subject an imperative leaves out.
			if c.Verb != nil && verbFollows(s, c) && imperativeTag(s, *c.Verb) {
				return true
			}
			// A finite verb right after the first word makes that word its subject: "; deliverable is prose".
			return !finiteAt(s, i+2) && opensImperative(s.Text[s.Words[i+1].Start:])
		}
		// "; when the cache is cold, the build waits": the subordinator took the link.
		if c.Kind == syntax.Subordinate && c.Link == i+1 {
			if k+1 < len(s.Clauses) {
				if next := s.Clauses[k+1]; next.Kind == syntax.Opens && next.Depth == 0 && next.Subject != nil && next.Verb != nil {
					return true
				}
			}
			// "; when the gap is fixed, return the verdict": the parser keeps an imperative in the subordinate clause.
			comma := firstCommaFrom(s, i+1)
			return comma > 0 && comma+1 < len(s.Words) && (imperativeTag(s, syntax.Phrase{First: comma + 1, Last: comma + 1}) || opensImperative(s.Text[s.Words[comma+1].Start:]))
		}
	}
	return false
}

// finiteAt reports a finite verb at word i of s.
func finiteAt(s *syntax.Sentence, i int) bool {
	if i < 0 || i >= len(s.Words) {
		return false
	}
	switch s.Words[i].Tag {
	case "VBZ", "VBP", "VBD", "MD":
		return true
	}
	return false
}

// imperativeTag reports a verb group that leads with a bare verb. The tagger
// often reads a bare verb with no subject as present tense.
func imperativeTag(s *syntax.Sentence, verb syntax.Phrase) bool {
	for j := verb.First; j <= verb.Last; j++ {
		switch s.Words[j].Tag {
		case "VB", "VBP":
			return true
		case "RB":
			continue
		}
		return false
	}
	return false
}

// fixSemicolons repairs every semicolon. One between clauses becomes a period.
// One between the items of a list after a colon opens a sentence of its own,
// "This also covers <item>". One before a clause after a phrase becomes a
// period too, because the phrase was a fragment as written. Any other joins
// its phrases with a comma. The blank after each is read from the text,
// because the mask writes a blank over the backtick of a code span.
func fixSemicolons(text string) string {
	text = pairLists(text)
	masked := checkMask(text)
	periods := set.Of(semicolonJoins(masked)...)
	var joins [][]int
	var openers []string
	var commas []int
	for _, span := range sentenceSpans(masked) {
		var marks []int
		for p := span[0]; p < span[1]; p++ {
			if masked[p] == ';' {
				marks = append(marks, p)
			}
		}
		for _, p := range marks {
			end := p + len(semicolonRun.FindString(text[p:]))
			switch {
			case periods.Contains(p):
				joins, openers = append(joins, []int{p, end}), append(openers, "")
			case strings.Contains(masked[span[0]:p], ":") && !strings.Contains(masked[p:span[1]], ":"):
				// The list's conjunction goes with the semicolon: "This also covers or a subsystem" is no English.
				end += len(leadingConjunction.FindString(text[end:]))
				joins, openers = append(joins, []int{p, end}), append(openers, "This also covers")
			case end < span[1] && standsAsSentence(segmentAfter(text, end, span[1])):
				joins, openers = append(joins, []int{p, end}), append(openers, "")
			case end < span[1] && holdsFinite(segmentAfter(text, end, span[1])) && holdsFinite(masked[span[0]:p]):
				// Words with a verb on each side are clauses the parser misread, as with a subject that a relative clause makes long.
				joins, openers = append(joins, []int{p, end}), append(openers, "")
			default:
				commas = append(commas, p)
			}
		}
	}
	sort.Ints(commas)
	// The commas go first, from the back, so every offset a join holds stays valid.
	for i := len(commas) - 1; i >= 0; i-- {
		p, join := commas[i], ", "
		run := semicolonRun.FindString(text[p:])
		text = text[:p] + join + text[p+len(run):]
		shift := len(join) - len(run)
		for j := range joins {
			if joins[j][0] > p {
				joins[j][0] += shift
				joins[j][1] += shift
			}
		}
	}
	return breakWith(text, joins, openers)
}

// leadingConjunction is the "and" or "or" that opens the last item of a list.
var leadingConjunction = regexp.MustCompile(`^(?:and|or)[ \t]+`)

// pairLists rewrites each list whose items open on the same preposition and
// pair a case with its answer after a comma: "for a parser, round-trip; for a
// store, get-after-set" becomes "for a parser (round-trip) and for a store
// (get-after-set)". The parenthesis keeps each answer with its case, where a
// comma would run the pairs together.
func pairLists(text string) string {
	masked := checkMask(text)
	spans := sentenceSpans(masked)
	for k := len(spans) - 1; k >= 0; k-- {
		span := spans[k]
		var marks []int
		for p := span[0]; p < span[1]; p++ {
			if masked[p] == ';' {
				marks = append(marks, p)
			}
		}
		lead, ok := prepositional(text, marks, span[1])
		if !ok {
			continue
		}
		first := lastWord(strings.ToLower(text[span[0]:marks[0]]), lead)
		if first < 0 || !strings.Contains(text[span[0]+first:marks[0]], ",") {
			continue
		}
		bounds := []int{span[0] + first}
		for _, p := range marks {
			bounds = append(bounds, p, p+len(semicolonRun.FindString(text[p:])))
		}
		end := span[1]
		for end > bounds[len(bounds)-1] && strings.ContainsAny(text[end-1:end], ".!? \t") {
			end--
		}
		bounds = append(bounds, end)
		var items []string
		for i := 0; i+1 < len(bounds); i += 2 {
			items = append(items, pairItem(strings.TrimSpace(text[bounds[i]:bounds[i+1]])))
		}
		joined := strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
		if len(items) > 2 {
			joined = strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
		}
		text = text[:bounds[0]] + joined + text[end:]
	}
	return text
}

// pairItem writes "for X, Y" as "for X (Y)". An answer that holds a
// parenthesis of its own keeps its comma.
func pairItem(item string) string {
	comma := strings.IndexByte(item, ',')
	answer := strings.TrimSpace(item[comma+1:])
	if comma < 0 || strings.ContainsAny(answer, "()") {
		return item
	}
	return item[:comma] + " (" + answer + ")"
}

func lastWord(text, w string) int {
	for at := len(text); at > 0; {
		i := strings.LastIndex(text[:at], w)
		if i < 0 {
			return -1
		}
		end := i + len(w)
		if (i == 0 || !unicode.IsLetter(rune(text[i-1]))) && (end == len(text) || !unicode.IsLetter(rune(text[end]))) {
			return i
		}
		at = i
	}
	return -1
}

// prepositional answers the preposition that opens every item after the
// semicolons, where each pairs it with an answer after a comma.
func prepositional(text string, marks []int, end int) (string, bool) {
	if len(marks) == 0 {
		return "", false
	}
	lead := ""
	for _, p := range marks {
		seg := segmentAfter(text, p+1, end)
		word := strings.ToLower(firstToken.FindString(seg))
		if !pairPrepositions.Contains(word) || !strings.Contains(seg, ",") || lead != "" && word != lead {
			return "", false
		}
		lead = word
	}
	return lead, true
}

// pairPrepositions open the case of a pair such as "for a parser, round-trip".
var pairPrepositions = set.Of("for", "in", "on", "at", "with", "by", "from", "to", "under", "within", "without", "via", "per", "inside")

// holdsFinite reports text with a finite verb or a modal outside a parenthesis.
func holdsFinite(text string) bool {
	return finiteBefore(syntax.Parse(checkMask(text), nil), 0, "")
}

// standsAsSentence reports text that reads as a sentence: a clause, an
// instruction, or a subordinate clause followed by either.
func standsAsSentence(text string) bool {
	if bareNounPhrase(text) {
		return false
	}
	if StandsAlone(text) || opensImperative(text) {
		return true
	}
	s := syntax.Parse(checkMask(text), nil)
	// A gerund subject the parser missed: "deciding whether X can learn anything needs Y".
	if len(s.Words) > 0 && (s.Words[0].Tag == "VBG" || s.Words[0].Tag == "VBN" || s.Words[0].Tag == "VBD") && finiteBefore(s, 1, ",") {
		return true
	}
	if !opensDependent(s) {
		return false
	}
	comma := firstComma(s)
	if comma < 0 || comma+1 >= len(s.Words) {
		return false
	}
	rest := strings.TrimSpace(text[s.Words[comma].End:])
	return StandsAlone(rest) || opensImperative(rest)
}

// bareNounPhrase reports text that opens on a comparison and a singular noun
// with no determiner: "richest signal for the pattern". It is an appositive.
func bareNounPhrase(text string) bool {
	words := syntax.Parse(checkMask(text), nil).Words
	return len(words) > 1 && (words[0].Tag == "JJS" || words[0].Tag == "JJR") && strings.HasPrefix(words[1].Tag, "NN")
}

// segmentAfter answers the text from byte from to the next semicolon or to end.
func segmentAfter(text string, from, end int) string {
	seg := text[from:end]
	if i := strings.IndexByte(seg, ';'); i >= 0 {
		seg = seg[:i]
	}
	return strings.TrimSpace(seg)
}
