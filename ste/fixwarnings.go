package ste

import (
	"fmt"
	"slices"
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
		text = fixClauseWarnings(text, keep)
	}
	if keep(IDInstructionLength) {
		text = fixInstructions(text, reorder)
	}
	return text
}

// clauseRounds bounds the clause rewrites for each word of the text, because a rewrite can hand a later clause a form the next round reads.
const clauseRounds = 8

// plain is the banned words a repair can write an approved word for, from the
// table's plain list. The dictionary as a whole cannot drive a repair, because
// a spelling does not tell one sense of a word from another. Each entry is a
// dictionary word, and its approved word is one the dictionary gives for it.
var plain = func() map[string]string {
	out := map[string]string{}
	for _, line := range steTable.List("plain") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			panic(fmt.Sprintf("rules/ste-words.xml: plain line %q is not a word and its approved word", line))
		}
		if !slices.Contains(dictionary[fields[0]], fields[1]) {
			panic(fmt.Sprintf("rules/ste-words.xml: plain line %q names no approved word the dictionary gives", line))
		}
		out[fields[0]] = fields[1]
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
		replacement, approved := plainSwap(prose, loc)
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

// plainSwap answers the approved word for the word at loc in prose. "more"
// takes no determiner, so "an additional step" and "per additional row" keep
// their word.
func plainSwap(prose string, loc []int) (string, bool) {
	word := prose[loc[0]:loc[1]]
	replacement, ok := plain[strings.ToLower(word)]
	if !ok {
		return "", false
	}
	// A word joined by a hyphen is part of a compound, as in "byte-identical".
	if loc[0] > 0 && prose[loc[0]-1] == '-' || loc[1] < len(prose) && prose[loc[1]] == '-' {
		return "", false
	}
	prev := strings.ToLower(previousWord(prose[:loc[0]]))
	if replacement == "more" && takesDeterminer.Contains(prev) {
		return "", false
	}
	// "same" reads only after "the": "emits identical ISA" cannot become "emits same ISA".
	if replacement == "same" && prev != "the" {
		return "", false
	}
	// "a wrong" cannot become "a incorrect", so the swap keeps the article right or does not happen.
	if (prev == "a" || prev == "an") && startsWithVowel(word) != startsWithVowel(replacement) {
		return "", false
	}
	return replacement, true
}

// startsWithVowel reports a word whose first letter is a vowel.
func startsWithVowel(word string) bool {
	return word != "" && strings.ContainsRune("aeiouAEIOU", rune(word[0]))
}

// takesDeterminer are the words before which "more" is not English.
var takesDeterminer = set.Of("a", "an", "the", "per", "each", "every", "any", "this", "that", "its", "their", "our", "one")

// previousWord answers the last word of text.
func previousWord(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[len(fields)-1], "*_(\"'")
}

// fixParagraphs writes the parts of Paragraphs with a blank line between them.
func fixParagraphs(text string) string {
	return strings.Join(Paragraphs(text), "\n\n")
}

// Paragraphs divides text into parts of ParagraphSentenceCap sentences or
// less, at the whitespace between sentences. A short text is one part.
func Paragraphs(text string) []string {
	spans := sentenceSpans(text)
	if len(spans) <= ParagraphSentenceCap {
		return []string{text}
	}
	var out []string
	last := 0
	for n, span := range spans {
		if n == 0 || n%ParagraphSentenceCap != 0 {
			continue
		}
		cut := span[0]
		for cut > last && strings.ContainsRune(" \t\n", rune(text[cut-1])) {
			cut--
		}
		out = append(out, text[last:cut])
		last = span[0]
	}
	return append(out, text[last:])
}

// fixClauseWarnings rewrites each sentence a clause warning reports, until no
// clause of the text carries one.
func fixClauseWarnings(prose string, keep func(id string) bool) string {
	var out strings.Builder
	last := 0
	for _, span := range sentenceSpans(prose) {
		out.WriteString(prose[last:span[0]])
		out.WriteString(fixSentenceWarnings(prose[span[0]:span[1]], keep))
		last = span[1]
	}
	out.WriteString(prose[last:])
	return out.String()
}

// fixSentenceWarnings rewrites one sentence until no clause warning its rules
// repair is left in it. A rewrite stays inside its sentence.
func fixSentenceWarnings(text string, keep func(id string) bool) string {
	for range clauseRounds * (len(strings.Fields(text)) + 1) {
		masked := mask(text)
		off := opaque(text, masked)
		s := syntax.Parse(masked, off)
		widenMasked(s, text)
		rewritten, ok := rewriteSentence(s, text, off, keep)
		if !ok || rewritten == text {
			break
		}
		text = rewritten
	}
	return text
}

// widenMasked gives each word that mask wrote over a code span, an entity or a
// link the whole span in source. A splice then moves the span whole.
func widenMasked(s *syntax.Sentence, source string) {
	spans := verbatimSpan.FindAllStringIndex(source, -1)
	for k := range s.Words {
		for _, sp := range spans {
			if sp[0] <= s.Words[k].Start && s.Words[k].Start < sp[1] {
				s.Words[k].Start, s.Words[k].End = sp[0], sp[1]
				break
			}
		}
	}
}

// fixInstructions divides each instruction over InstructionWordCap, as the
// sentence cap divides a sentence over its own cap.
func fixInstructions(prose string, reorder bool) string {
	spans := sentenceSpans(prose)
	for n := len(spans) - 1; n >= 0; n-- {
		text := prose[spans[n][0]:spans[n][1]]
		masked := mask(text)
		count := WordCount(masked)
		if count <= InstructionWordCap || count > SentenceWordCap || !isInstruction(syntax.Parse(masked, opaque(text, masked))) {
			continue
		}
		prose = prose[:spans[n][0]] + fixSentenceCap(text, capSpec{reorder: reorder, cap: InstructionWordCap}) + prose[spans[n][1]:]
	}
	return prose
}

// rewriteSentence answers the sentence with the first clause warning its own
// rule can repair, in the order the repairs read the words.
func rewriteSentence(s *syntax.Sentence, source string, off [][]int, keep func(id string) bool) (string, bool) {
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
		if out, ok := rewriteCluster(s, source, off); ok {
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
	if !simpleSpan(s, i, j) {
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
	// The actor is one noun phrase. A verb or a conjunction in it means the clause goes on past the actor.
	for k := by + 1; k <= end; k++ {
		if tag := s.Words[k].Tag; tag == "CC" || strings.HasPrefix(tag, "VB") || tag == "MD" {
			return "", false
		}
	}
	// The words between the participle and "by" go after the object.
	between := strings.TrimSpace(source[s.Words[j].End:s.Words[by].Start])
	if strings.ContainsAny(between, ",;()[]—") || hasConjunction(s, j+1, by) {
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
	if between != "" {
		object += " " + between
	}
	return capitalizeOpening(actor) + " " + verb + " " + object + tail, true
}

// hasConjunction reports a coordinating conjunction among the words from
// first up to last.
func hasConjunction(s *syntax.Sentence, first, last int) bool {
	for k := first; k < last; k++ {
		if s.Words[k].Tag == "CC" {
			return true
		}
	}
	return false
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
	base := baseFromParticiple(participle)
	if base == "" {
		return ""
	}
	return presentOf(base, plural)
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
	if !certainStem(stem) {
		return ""
	}
	return stem
}

// simpleSpan reports whether a verb group from the auxiliary i to the
// participle j holds only the words the simple-tense repair replaces.
func simpleSpan(s *syntax.Sentence, i, j int) bool {
	return j == i+1
}

// rewriteTense writes a simple tense for a perfect or a progressive verb
// group: "has read" becomes "read", and "is stopping" becomes "stops".
// "has not read" is left as written, because the negative is not part of the
// tense and no rewrite here writes it back.
func rewriteTense(s *syntax.Sentence, source string) (string, bool) {
	if i, j := auxiliary(s, 0, haveForms); i >= 0 && s.Words[j].Tag == "VBN" {
		if !simpleSpan(s, i, j) || afterModal(s, i) {
			return "", false
		}
		return splice(source, s, i, j, pastFromParticiple(s.Words[j].Lower())), true
	}
	i, j := auxiliary(s, 1, beForms)
	// A hyphenated -ing word is an adjective, as in "load-bearing".
	if i < 1 || j < 0 || s.Words[j].Tag != "VBG" || approvedIng.Contains(s.Words[j].Lower()) || s.Words[j].Lower() == "being" ||
		strings.Contains(s.Words[j].Text, "-") {
		return "", false
	}
	if !simpleSpan(s, i, j) {
		return "", false
	}
	subject, ok := subjectBefore(s, i)
	if !ok {
		return "", false
	}
	base := baseFromGerund(s.Words[j].Lower())
	if base == "" {
		return "", false
	}
	return splice(source, s, i, j, presentOf(base, s.Plural(subject))), true
}

// afterModal reports a have-form that follows a modal.
func afterModal(s *syntax.Sentence, i int) bool {
	return i > 0 && (s.Words[i-1].Tag == "MD" || modalWords.Contains(s.Words[i-1].Lower()))
}

var modalWords = set.Of("can", "could", "will", "would", "shall", "should", "may", "might", "must")

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

// tenseAt answers the verb group rewriteTense rewrites, and whether it is a
// perfect tense rather than a progressive one.
func tenseAt(s *syntax.Sentence) (int, int, bool) {
	if i, j := auxiliary(s, 0, haveForms); i >= 0 && s.Words[j].Tag == "VBN" {
		return i, j, true
	}
	i, j := auxiliary(s, 1, beForms)
	return i, j, false
}

// clusterAt answers the first run of more than NounClusterCap nouns. Code, a
// quotation, a parenthetical and a bracket are data, so each ends a run.
func clusterAt(s *syntax.Sentence, off [][]int) (int, int, bool) {
	last := lastContent(s)
	noun := func(k int) bool {
		w := s.Words[k]
		return strings.HasPrefix(w.Tag, "NN") && !insideAny(off, w.Start) && !masks.Contains(w.Text) &&
			!strings.ContainsAny(w.Text, "()[]{}`\"") && strings.IndexFunc(w.Text, unicode.IsLetter) >= 0
	}
	// The repair can only trust a run of plain lowercase words that a determiner opens.
	// A capital, a digit, a hyphen or a slash marks a name or a compound. A run with no
	// determiner in front often holds a verb the tagger read as a noun.
	plainRun := func(start, end int) bool {
		if start == 0 || s.Words[start-1].Tag != "DT" && s.Words[start-1].Tag != "PRP$" && s.Words[start-1].Tag != "POS" {
			return false
		}
		for k := start; k <= end; k++ {
			if strings.IndexFunc(s.Words[k].Text, func(r rune) bool { return r < 'a' || r > 'z' }) >= 0 {
				return false
			}
		}
		return true
	}
	for start := 0; start <= last; start++ {
		if !noun(start) {
			continue
		}
		end := start
		for end+1 <= last && noun(end+1) {
			end++
		}
		if nameUnits(s, start, end) > NounClusterCap && plainRun(start, end) {
			return start, end, true
		}
		start = end
	}
	return -1, -1, false
}

// rewriteCluster breaks a run of nouns apart: "the gate file system cache
// lookup" becomes "the lookup of the gate file system cache". A longer run
// keeps taking its last noun as the head until at most NounClusterCap nouns
// stand together, all in one rewrite.
func rewriteCluster(s *syntax.Sentence, source string, off [][]int) (string, bool) {
	start, end, ok := clusterAt(s, off)
	if !ok {
		return "", false
	}
	var out strings.Builder
	last := end
	// The head names the thing, and the nouns in front of it describe it. A name is one head.
	for nameUnits(s, start, last) > NounClusterCap {
		head := nameStart(s, start, last)
		out.WriteString(source[s.Words[head].Start:s.Words[last].End])
		out.WriteString(" of the ")
		last = head - 1
	}
	out.WriteString(source[s.Words[start].Start:s.Words[last].End])
	return splice(source, s, start, end, out.String()), true
}

// nameUnits counts the nouns from start to end, with each run of capitalized
// words as one noun. "AMD Vega Instruction Set" is one name.
func nameUnits(s *syntax.Sentence, start, end int) int {
	n := 0
	for k := end; k >= start; k = nameStart(s, start, k) - 1 {
		n++
	}
	return n
}

// nameStart answers the first word of the run of capitalized words that ends
// at k. It k itself when the word at k is not capitalized.
func nameStart(s *syntax.Sentence, start, k int) int {
	if !isCapitalized(s.Words[k].Text) {
		return k
	}
	for k > start && isCapitalized(s.Words[k-1].Text) {
		k--
	}
	return k
}
