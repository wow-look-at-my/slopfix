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
		rewritten, ok := rewriteSentence(coverVerbatim(syntax.Parse(masked, off), text), text, off, keep)
		if !ok || rewritten == text {
			break
		}
		text = rewritten
	}
	return text
}

// coverVerbatim stretches each word mask writes over a code span, a link
// target or an entity to the whole span. A rewrite that moves the word then
// moves all of the span.
func coverVerbatim(s *syntax.Sentence, text string) *syntax.Sentence {
	for _, span := range verbatimSpan.FindAllStringIndex(text, -1) {
		from, to := span[0], span[1]
		// mask fills a link target inside its parentheses.
		if to-from >= 3 && text[from] == ']' && text[from+1] == '(' && text[to-1] == ')' {
			from, to = from+2, to-1
		}
		for k := range s.Words {
			if s.Words[k].Start >= from && s.Words[k].Start < to {
				s.Words[k].Start, s.Words[k].End = from, to
			}
		}
	}
	return s
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
		if endsActor(s.Words[k]) {
			end = k - 1
			break
		}
	}
	if end < by+1 {
		return "", false
	}
	// "by" and a gerund name the means, and no actor: "is asserted by pairing it with X".
	if s.Words[by+1].Tag == "VBG" {
		return "", false
	}
	// A second verb group after the actor shares the passive subject, which the rewrite moves away from it: "is untouched by this and is now the slowest".
	for k := by + 2; k < end; k++ {
		if s.Words[k].Tag == "CC" && (strings.HasPrefix(s.Words[k+1].Tag, "VB") || s.Words[k+1].Tag == "MD") {
			return "", false
		}
	}
	first, ok := subjectStart(s, i)
	if !ok {
		return "", false
	}
	from, to := s.Words[first].Start, s.Words[end].End
	subject := strings.TrimSpace(unbold(source[from:s.Words[i].Start]))
	actor := strings.TrimSpace(unbold(source[s.Words[by+1].Start:to]))
	verb := passiveVerb(s.Words[j].Lower(), s.Words[i].Lower(), s.Plural(syntax.Phrase{Head: end}))
	if subject == "" || actor == "" || verb == "" {
		return "", false
	}
	object := subject
	if first == 0 {
		actor = capitalizeOpening(actor)
		object = lowerOpeningFor(subject, s.Words[0].Tag)
	}
	clause := rebold(source, from, to, actor+" "+verb+" "+object)
	return source[:from] + clause + source[to:], true
}

// endsActor reports a word the actor of a passive clause stops in front of.
// Punctuation ends it, and so does a subordinator.
func endsActor(w syntax.Word) bool {
	switch w.Text {
	case ",", ";", ":", "(":
		return true
	}
	if strings.Trim(w.Text, "-–—") == "" {
		return true
	}
	return syntax.Is(w.Lower(), "subordinator")
}

// subjectStart answers the first word of the subject in front of the verb
// group at i. An opening phrase a comma closes stays in front of the clause, as
// does the coordinator or subordinator after that comma. A relative pronoun
// stands for a noun outside the clause, so it has no subject to move.
func subjectStart(s *syntax.Sentence, i int) (int, bool) {
	first := 0
	for k := 0; k < i; k++ {
		if s.Words[k].Text == "," && leadsClause(s, first, k) {
			first = k + 1
		}
	}
	for first < i && (s.Words[first].Tag == "CC" || syntax.Is(s.Words[first].Lower(), "subordinator") || s.Words[first].Lower() == "so") {
		first++
	}
	// A subject opens on a noun phrase. A verb there is a reduced clause of the words before: "run by the action the same way X is".
	if first >= i || strings.HasPrefix(s.Words[first].Tag, "W") || strings.HasPrefix(s.Words[first].Tag, "VB") {
		return 0, false
	}
	return first, true
}

// leadsClause reports whether the words from first up to the comma at k stand
// in front of a clause rather than in a list. They carry a finite verb, or they
// open on a preposition or an adverb, or the comma is followed by a coordinator
// or a subordinator.
func leadsClause(s *syntax.Sentence, first, k int) bool {
	if k+1 < len(s.Words) {
		next := s.Words[k+1]
		if next.Tag == "CC" || next.Lower() == "so" || syntax.Is(next.Lower(), "subordinator") {
			return true
		}
	}
	if first < k {
		switch s.Words[first].Tag {
		case "IN", "RB", "TO", "VBG":
			return true
		}
	}
	for _, w := range s.Words[first:k] {
		switch w.Tag {
		case "VBZ", "VBP", "VBD", "MD":
			return true
		}
	}
	return false
}

// unbold drops the bold markers from a piece a rewrite moves.
func unbold(piece string) string {
	return strings.ReplaceAll(piece, "**", "")
}

// rebold writes the bold markers source carries between from and to around the
// rewritten clause. A run that opens inside the clause opens at its start, and
// a run that closes inside it closes at its end.
func rebold(source string, from, to int, clause string) string {
	inside := strings.Count(source[from:to], "**")
	if inside == 0 {
		return clause
	}
	startBold := strings.Count(source[:from], "**")%2 == 1
	endBold := (strings.Count(source[:from], "**")+inside)%2 == 1
	if !startBold {
		clause = "**" + clause
	}
	if !endBold {
		clause += "**"
	}
	return clause
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
	if silentE(stem) {
		return stem + "e"
	}
	return stem
}

// silentE reports whether English spelling drops an e from the end of stem before
// -ed. Endings like -dge, -ate, -ize, -ve, -ce and -ure need the e. A stem of one
// syllable that ends on a single vowel and a single consonant took the e as well.
// Because a stem without it doubles its consonant: "named" against "planned".
func silentE(stem string) bool {
	n := len(stem)
	if n < 2 {
		return false
	}
	last, prev := stem[n-1], stem[n-2]
	consonant := func(c byte) bool { return c >= 'a' && c <= 'z' && !strings.ContainsRune("aeiou", rune(c)) }
	switch {
	case last == 'v', last == 'u' && prev != 'q':
		return true
	case last == 'c', last == 'z' && strings.ContainsRune("iy", rune(prev)):
		return true
	case last == 'g' && prev == 'd':
		return true
	case last == 's' && strings.ContainsRune("rnpl", rune(prev)):
		return true
	case last == 'l' && consonant(prev) && !strings.ContainsRune("lrw", rune(prev)):
		return true
	case n >= 3 && strings.ContainsRune("aiu", rune(prev)) && strings.ContainsRune("tr", rune(last)) && consonant(stem[n-3]):
		// -ate, -ire, -ure and -are, after a consonant.
		return prev != 'i' || last == 'r'
	case n >= 3 && prev == 'i' && strings.ContainsRune("nd", rune(last)) && consonant(stem[n-3]):
		// -ine and -ide, after a consonant.
		return true
	}
	// The u of qu spells a consonant sound.
	w := strings.ReplaceAll(stem, "qu", "q")
	n = len(w)
	return n >= 2 && oneSyllable(w) && strings.ContainsRune("aeiou", rune(w[n-2])) && (n == 2 || consonant(w[n-3])) &&
		consonant(last) && !strings.ContainsRune("wxy", rune(last))
}

// oneSyllable reports whether word holds a single run of vowels. A y opens no
// run at the start of a word, where it spells a consonant.
func oneSyllable(word string) bool {
	runs := 0
	vowel := false
	for i := 0; i < len(word); i++ {
		v := strings.ContainsRune("aeiou", rune(word[i])) || word[i] == 'y' && i > 0
		if v && !vowel {
			runs++
		}
		vowel = v
	}
	return runs == 1
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
			!strings.ContainsAny(w.Text, "()[]{}`\"") && !versus.Contains(w.Lower())
	}
	for start := 0; start <= last; start++ {
		if !noun(start) {
			continue
		}
		end := start
		for end+1 <= last && noun(end+1) {
			end++
		}
		if end-start+1 > NounClusterCap {
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
	// The head names the thing, and the nouns in front of it describe it.
	for last-start+1 > NounClusterCap {
		out.WriteString(source[s.Words[last].Start:s.Words[last].End])
		out.WriteString(" of the ")
		last--
	}
	rest := source[s.Words[start].Start:s.Words[last].End]
	rewritten := out.String()
	// A cluster that opens the sentence hands its capital to the head that now opens it.
	if start == 0 && isCapitalized(s.Words[0].Text) {
		rewritten = capitalizeOpening(rewritten)
		rest = lowerOpeningFor(rest, s.Words[0].Tag)
	}
	return splice(source, s, start, end, rewritten+rest), true
}

// versus joins noun phrases and is no noun of either, though the tagger reads it as one.
var versus = set.Of("vs", "vs.", "versus")
