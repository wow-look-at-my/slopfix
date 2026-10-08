package ste

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// The repairs the warning rules answer with. A warning informs rather than
// fails a check, and `slopfix fix` still rewrites it. Each repair here clears
// the finding its own rule reports on the rule's own case.

// fixWarnings applies every warning repair the caller keeps, in the order the
// repairs read one another's output.
func fixWarnings(text string, keep func(id string) bool, reorder bool) string {
	if keep(IDParagraphLength) {
		text = fixParagraphs(text)
	}
	if keep(IDDictionary) {
		text = fixProse(text, widenDictionary)
	}
	if keep(IDTense) || keep(IDPassive) || keep(IDNounCluster) {
		text = fixProse(text, func(prose string) string { return fixClauseWarnings(prose, keep) })
	}
	if keep(IDInstructionLength) {
		text = fixSentenceCap(text, capSpec{reorder: reorder, cap: InstructionWordCap})
	}
	return text
}

// clauseRounds bounds the clause repairs, because a rewrite can hand a later clause a form the next round reads.
const clauseRounds = 8

// plain is the banned words a repair can write an approved word for, from the
// table's plain list. The dictionary as a whole cannot drive a repair, because
// a spelling does not tell one sense of a word from another.
var plain = func() map[string]string {
	out := map[string]string{}
	for _, line := range steTable.List("plain") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			out[fields[0]] = fields[1]
		}
	}
	return out
}()

// widenDictionary writes the approved word for each word the STE dictionary
// does not approve, and keeps the capitalization the source used.
func widenDictionary(prose string) string {
	locs := wordPattern.FindAllStringIndex(prose, -1)
	var out strings.Builder
	last := 0
	for _, loc := range locs {
		out.WriteString(prose[last:loc[0]])
		last = loc[1]
		word := prose[loc[0]:loc[1]]
		if from, to := loc[0], loc[1]; from > 0 && isIdentByte(prose[from-1]) || to < len(prose) && isIdentByte(prose[to]) {
			out.WriteString(word)
			continue
		}
		replacement, approved := plain[strings.ToLower(word)]
		if !approved {
			out.WriteString(word)
			continue
		}
		if isCapitalized(word) {
			replacement = capitalize(replacement)
		}
		out.WriteString(replacement)
	}
	out.WriteString(prose[last:])
	return out.String()
}

// fixParagraphs divides a paragraph over the sentence cap into paragraphs, so
// no paragraph carries more sentences than the rule allows. A short paragraph
// is returned as written.
func fixParagraphs(text string) string {
	spans := sentenceSpans(text)
	if len(spans) <= ParagraphSentenceCap {
		return text
	}
	var out strings.Builder
	last := 0
	for n, span := range spans {
		if n == 0 || n%ParagraphSentenceCap != 0 {
			continue
		}
		// The break lands on the whitespace that separates the sentences.
		cut := span[0]
		for cut > last && strings.ContainsRune(" \t\n", rune(text[cut-1])) {
			cut--
		}
		out.WriteString(text[last:cut])
		out.WriteString("\n\n")
		last = span[0]
	}
	out.WriteString(text[last:])
	return out.String()
}

// fixClauseWarnings rewrites each sentence a clause warning reports, until no
// clause of the text carries one.
func fixClauseWarnings(prose string, keep func(id string) bool) string {
	for range clauseRounds {
		next, changed := rewriteWarningClause(prose, keep)
		if !changed {
			break
		}
		prose = next
	}
	return prose
}

// rewriteWarningClause rewrites the first sentence that carries a clause
// warning, and reports whether it changed anything.
func rewriteWarningClause(prose string, keep func(id string) bool) (string, bool) {
	for _, span := range sentenceSpans(prose) {
		text := prose[span[0]:span[1]]
		masked := mask(text)
		s := syntax.Parse(masked, opaque(text, masked))
		rewritten, ok := rewriteSentence(s, text, keep)
		if !ok || rewritten == text {
			continue
		}
		return prose[:span[0]] + rewritten + prose[span[1]:], true
	}
	return prose, false
}

// rewriteSentence answers the sentence with the first clause warning its own
// rule can repair, in the order the repairs read the words.
func rewriteSentence(s *syntax.Sentence, source string, keep func(id string) bool) (string, bool) {
	if keep(IDPassive) {
		if out, ok := rewritePassive(s, source); ok {
			return out, true
		}
	}
	if keep(IDTense) {
		if out, ok := rewriteTense(s, source); ok {
			return out, true
		}
	}
	if keep(IDNounCluster) {
		if out, ok := rewriteCluster(s, source); ok {
			return out, true
		}
	}
	return source, false
}

// lastContent answers the last word of a sentence that is not punctuation.
func lastContent(s *syntax.Sentence) int {
	last := len(s.Words) - 1
	for last > 0 && punctuationTag(s.Words[last].Tag) {
		last--
	}
	return last
}

// auxiliary answers the word that opens a verb group, skipping an adverb or a
// negative that stands between the auxiliary and its participle.
func auxiliary(s *syntax.Sentence, from int, forms set.Set[string]) (int, int) {
	for i := from; i < len(s.Words); i++ {
		if !forms.Contains(s.Words[i].Lower()) {
			continue
		}
		next := i + 1
		for next < len(s.Words) && (s.Words[next].Tag == "RB" || s.Words[next].Lower() == "not") {
			next++
		}
		if next < len(s.Words) {
			return i, next
		}
	}
	return -1, -1
}

// rewritePassive writes the active voice for a passive clause that names its
// actor: "The file is read by the gate" becomes "The gate reads the file".
// A clause that names no actor is left as written, because nothing states who
// acts.
func rewritePassive(s *syntax.Sentence, source string) (string, bool) {
	last := lastContent(s)
	i, j := auxiliary(s, 1, beForms)
	if i < 1 || j < 0 || s.Words[j].Tag != "VBN" {
		return "", false
	}
	// The actor is the phrase after "by", and it must close the clause.
	by := -1
	for k := j + 1; k < last; k++ {
		if s.Words[k].Lower() == "by" {
			by = k
			break
		}
	}
	if by < 0 || by+1 > last {
		return "", false
	}
	end := last
	for k := by + 1; k <= last; k++ {
		if s.Words[k].Text == "," {
			end = k - 1
			break
		}
	}
	if end < by+1 {
		return "", false
	}
	subject := strings.TrimSpace(source[s.Words[0].Start:s.Words[i].Start])
	actor := source[s.Words[by+1].Start:s.Words[end].End]
	tail := source[s.Words[end].End:]
	verb := passiveVerb(s.Words[j].Lower(), s.Words[i].Lower(), s.Plural(syntax.Phrase{Head: end}))
	if subject == "" || actor == "" || verb == "" {
		return "", false
	}
	object := lowerOpeningFor(subject, s.Words[0].Tag)
	return capitalizeOpening(actor) + " " + verb + " " + object + tail, true
}

// lowerOpeningFor writes a phrase as it reads inside a sentence rather than at
// the start of one. A name keeps its capital.
func lowerOpeningFor(phrase, tag string) string {
	if tag == "NNP" || tag == "NNPS" {
		return phrase
	}
	first, width := utf8.DecodeRuneInString(phrase)
	if !unicode.IsUpper(first) {
		return phrase
	}
	return string(unicode.ToLower(first)) + phrase[width:]
}

// passiveVerb answers the active form of a passive verb group: the participle
// for a past clause, and the present tense of the base form otherwise.
func passiveVerb(participle, beWord string, plural bool) string {
	switch beWord {
	case "was", "were", "been", "being":
		return pastFromParticiple(participle)
	case "be":
		return pastFromParticiple(participle)
	}
	return presentOf(baseFromParticiple(participle), plural)
}

// baseFromParticiple answers the verb a participle belongs to. A regular verb
// spells its participle with -ed, so the ending answers it by rule. The table
// answers each verb the ending hides.
func baseFromParticiple(form string) string {
	form = strings.ToLower(form)
	if v, ok := verbBy(form); ok && v.participle == form {
		return v.base
	}
	if !strings.HasSuffix(form, "ed") || len(form) <= 3 {
		return form
	}
	stem := form[:len(form)-2]
	if doubled(stem) {
		return stem[:len(stem)-1]
	}
	if strings.HasSuffix(stem, "i") {
		return stem[:len(stem)-1] + "y"
	}
	if v, ok := verbBy(stem + "e"); ok && v.base == stem+"e" {
		return stem + "e"
	}
	return stem
}

// rewriteTense writes a simple tense for a perfect or a progressive verb
// group: "has read" becomes "read", and "is stopping" becomes "stops".
func rewriteTense(s *syntax.Sentence, source string) (string, bool) {
	if i, j := auxiliary(s, 0, haveForms); i >= 0 && s.Words[j].Tag == "VBN" {
		return splice(source, s, i, j, pastFromParticiple(s.Words[j].Lower())), true
	}
	i, j := auxiliary(s, 1, beForms)
	if i < 1 || j < 0 || s.Words[j].Tag != "VBG" || approvedIng.Contains(s.Words[j].Lower()) || s.Words[j].Lower() == "being" {
		return "", false
	}
	subject, ok := subjectBefore(s, i)
	if !ok {
		return "", false
	}
	return splice(source, s, i, j, presentOf(baseFromGerund(s.Words[j].Lower()), s.Plural(subject))), true
}

// subjectBefore answers the noun phrase that stands in front of word i, which
// is the subject the verb of a progressive clause agrees with.
func subjectBefore(s *syntax.Sentence, i int) (syntax.Phrase, bool) {
	for k := i - 1; k >= 0; k-- {
		if p, ok := s.PhraseAt(k); ok && p.Kind == syntax.NounPhrase && p.Last < i {
			return *p, true
		}
	}
	return syntax.Phrase{}, false
}

// splice writes replacement in place of the words from first to last, keeping
// the source on each side.
func splice(source string, s *syntax.Sentence, first, last int, replacement string) string {
	start, end := s.Words[first].Start, s.Words[last].End
	return source[:start] + replacement + source[end:]
}

// rewriteCluster breaks a run of nouns apart: "the gate file system cache
// lookup" becomes "the lookup of the gate file system cache".
func rewriteCluster(s *syntax.Sentence, source string) (string, bool) {
	last := lastContent(s)
	for start := 0; start <= last; start++ {
		if !strings.HasPrefix(s.Words[start].Tag, "NN") {
			continue
		}
		end := start
		for end+1 <= last && strings.HasPrefix(s.Words[end+1].Tag, "NN") {
			end++
		}
		if end-start+1 < NounClusterCap+1 {
			start = end
			continue
		}
		// The head names the thing, and the nouns in front of it describe it.
		head := source[s.Words[end].Start:s.Words[end].End]
		modifiers := source[s.Words[start].Start:s.Words[end-1].End]
		return splice(source, s, start, end, head+" of the "+modifiers), true
	}
	return "", false
}
