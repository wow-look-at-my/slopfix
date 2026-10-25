// carrier.go divides a long sentence where the words after the cut hold no clause.

package ste

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// carrier.go divides a long sentence where the words after the cut hold no
// clause. Those words are a trailing adverbial, a phrase that describes a
// noun, or the rest of a list.

var (
	// carrierAdverbial words open an adverbial that a carrier can take.
	carrierAdverbial = set.Of(wordsOf("carrier-adverbial")...)
	// focusing adverbs bind to the phrase after them: "lands only on the lines".
	focusing = set.Of("only", "just", "even", "also", "still", "exactly", "directly", "right", "mostly", "mainly", "solely",
		"immediately", "shortly", "soon", "long", "well", "far", "straight")
	// carrierBound words move behind a carrier only before "every" or "each".
	carrierBound = set.Of(wordsOf("carrier-bound")...)
	// carrierPlace words open a phrase of place, which describes the noun before it.
	carrierPlace = set.Of(wordsOf("carrier-place")...)
	// stateVerbs state a fact rather than an action.
	stateVerbs = set.Of(wordsOf("state-verb")...)
	// nominalIng words end a compound noun after a singular noun: "the ste finding".
	nominalIng = set.Of(wordsOf("nominal-ing")...)
	// partitives name a part of a group and take a plural verb: "the rest are".
	partitives = set.Of(wordsOf("partitive")...)
	// indefinites are pronouns that take no restating determiner: "That nothing is" reads as nonsense.
	indefinites = set.Of("nothing", "anything", "something", "everything", "none", "nobody", "anyone", "someone", "everyone")
	// negations turn a verb group around.
	negations = set.Of("not", "never", "n't", "no")
	// auxiliaries come before the verb they carry.
	auxiliaries = set.Of("do", "does", "did", "not", "never", "n't", "must", "can", "will", "may", "should", "cannot")
)

// opensWithCarrier is what a division behind a carrier adds to a cut's score.
const opensWithCarrier = -6

// carrierDivision writes the words after cut c as a sentence of their own
// behind a carrier. It answers the head the division keeps, the rest, and
// the score the rest adds. The rest is "" when no carrier fits.
func carrierDivision(source string, whole *syntax.Sentence, c forceCut) (string, string, int) {
	head := source[:c.left]
	rest := strings.TrimLeft(source[c.right:], " ")
	seam := seamBefore(source, c.left)
	if rest == "" || seam != "" && seam != "," && seam != ":" && !seamIsDash(seam) {
		return head, "", 0
	}
	first := wordFrom(whole, len(source)-len(rest))
	if first < 1 {
		return head, "", 0
	}
	word := whole.Words[first]
	lower := word.Lower()
	prev, ok := lastWordBefore(whole, c.left)
	if !ok {
		return head, "", 0
	}
	// A head that ends inside a clause still waiting for its verb is no sentence.
	if headOpen(whole, first) {
		return head, "", 0
	}
	// An aside that opens on a dash and closes on one, before "and <verb>",
	// divides into the aside and the verb group with the subject named. Again:
	// "X — A does Y — and yields Z" becomes "X. A does Y. It yields Z."
	if trimmed := strings.TrimLeft(rest, " "); strings.HasPrefix(trimmed, "—") {
		if body := strings.TrimLeft(trimmed[len("—"):], " "); body != "" {
			if end := strings.Index(body, "—"); end > 0 {
				aside := strings.TrimSpace(body[:end])
				if clause, ok := strings.CutPrefix(strings.TrimSpace(body[end+len("—"):]), "and "); ok && opensVerb(clause) {
					return head, capitalizeOpening(aside) + ". It " + lowerFirst(clause), opensWithCarrier
				}
			}
		}
	}
	// ", as is X" or ", as are X" compares X to the subject: "A is true, as is
	// B" becomes "A is true. The same holds for B."
	if seam == "," && lower == "as" {
		for _, lead := range []string{"as is", "as are"} {
			if body, ok := strings.CutPrefix(strings.TrimLeft(rest, " "), lead); ok {
				return head, "The same holds for " + strings.TrimSpace(body), opensWithCarrier
			}
		}
	}
	noun := strings.HasPrefix(prev.Tag, "NN")
	// An -ing noun after a singular noun is one compound noun with it, and no division lands inside it.
	if prev.Tag == "NN" && nominalIng.Contains(lower) {
		return head, "", 0
	}
	// "A trailing run of turns that each made a call, spaced close together": a participle after a fragment describes the fragment's head.
	if seam == "," && (word.Tag == "VBN" || word.Tag == "VBD") && first+1 < len(whole.Words) && !takesObject(whole, first+1) &&
		!finiteOutsideRelative(syntax.Parse(checkMask(head), nil), 0) {
		if subject, ok := fragmentSubject(source, whole); ok {
			return fragmentHead(head), subject + " " + rest, opensWithCarrier
		}
		// The head is a noun phrase the sentence opened with, so the noun the
		// participle describes is named again.
		if strings.HasPrefix(prev.Tag, "NN") && !indefinites.Contains(prev.Lower()) {
			return head, restate(source, prev, rest), opensWithCarrier
		}
	}
	// "close enough together that no event explains it": the that-clause after "enough" is a result, and it stands as a sentence of its own.
	if seam == "" && lower == "that" && enoughBefore(whole, first) {
		if result := capitalizeOpening(strings.TrimLeft(rest[len(word.Text):], " ")); StandsAlone(result) {
			return head, result, opensWithCarrier
		}
	}
	main, hasMain := mainVerb(whole, c.left)
	// ", rather than <participle>" contrasts the clause with what the subject
	// does not do: "X is refused, rather than written through" becomes "X is
	// refused. X is not written through".
	if seam == "," && lower == "rather" && first+1 < len(whole.Words) && whole.Words[first+1].Lower() == "than" {
		if body := strings.TrimSpace(strings.TrimPrefix(strings.TrimLeft(rest, " "), "rather than")); body != "" {
			subject := "It"
			if hasMain {
				if s := mainSubject(source, whole, main, "VBZ"); s != "" {
					subject = capitalizeOpening(s)
				}
			}
			return head, subject + " is not " + body, opensWithCarrier
		}
	}
	// A dash before a verb group, or before "and" and a verb group, closes the
	// clause before it and names the subject again. "One call — with no turn of
	// its own — leaves X" becomes "One call. It leaves X".
	if seamIsDash(seam) {
		// "— keeping it out here will let X": a gerund opens a subject of its
		// own, so the words after the dash stand without the subject named again.
		if w := whole.Words[first]; w.Tag == "VBG" && opensSubject(whole, first) && StandsAlone(capitalizeOpening(rest)) {
			return head, capitalizeOpening(rest), opensWithCarrier
		}
		switch {
		case whole.Words[first].Tag == "VBG":
			// "-- using hyperfine" says how the main verb acts: "It uses hyperfine".
			body, found := strings.CutPrefix(rest, word.Text)
			if !hasMain || !found {
				return head, "", 0
			}
			return head, "It " + presentOf(baseFromGerund(word.Text), false) + body, opensWithCarrier
		case verbAt(whole, first) && participleAfterDash(whole, first, main, hasMain):
			// "says — asserted as X" is passive: "It is asserted as X", not "It asserted as X".
			return head, "It is " + lowerFirst(rest), opensWithCarrier
		case verbAt(whole, first):
			return head, "It " + lowerFirst(rest), opensWithCarrier
		case (lower == "and" || lower == "or") && first+1 < len(whole.Words) && verbAt(whole, first+1):
			return head, "It " + lowerFirst(strings.TrimLeft(rest[len(word.Text):], " ")), opensWithCarrier
		}
	}
	// A comma before a preposition or a participle that describes the opening
	// noun phrase restates its head noun: "The same leniency over X, for Y"
	// becomes "That leniency is for Y".
	if seam == "," && (word.Tag == "IN" || word.Tag == "TO" || word.Tag == "VBN" || word.Tag == "VBG") &&
		!clauseOpeners.Contains(lower) && !opensAdverbialClause(whole, first) && first+1 < len(whole.Words) && opensObject(whole, first+1) {
		if noun, ok := headNoun(whole, c.left); ok {
			return head, restate(source, noun, rest), opensWithCarrier
		}
	}
	// A verb group after "and" shares the main clause's subject, so the subject
	// is named again: "Drives X and asserts Y" becomes "Drives X. It asserts Y".
	if (seam == "," || seam == "") && (lower == "and" || lower == "or") && strings.HasPrefix(prev.Tag, "NN") &&
		first+1 < len(whole.Words) && verbAt(whole, first+1) {
		clause := strings.TrimLeft(rest[len(word.Text):], " ")
		// "and setting both limits to 0 disables eviction". A gerund opens a
		// subject of its own, so the words after the conjunction stand as a
		// sentence without the main clause's subject named again.
		if w := whole.Words[first+1]; w.Tag == "VBG" && opensSubject(whole, first+1) && StandsAlone(capitalizeOpening(clause)) {
			return head, capitalizeOpening(clause), opensWithCarrier
		}
		if subject := subjectFor(source, checkMask(source), c, whole.Words[first+1].Tag); subject != "" {
			return head, capitalizeOpening(subject) + " " + clause, opensWithCarrier
		}
		return head, "It " + lowerFirst(clause), opensWithCarrier
	}
	// A phrase after a noun that a verb then follows is the subject's own: "the case above passes".
	predicateFollows := finiteBefore(whole, first+1, ",")
	placeless := predicateFollows || !opensObject(whole, first+1)
	// A phrase that describes the noun before it ends the sentence: no comma, conjunction or clause follows it.
	describes := seam == "" && noun && !indefinites.Contains(prev.Lower()) && plainPhrase(whole, first)
	switch {
	case !hasMain && !finiteBetween(whole, 0, len(whole.Words)):
		// A sentence with no verb at all is a noun phrase, and only a phrase that describes a noun in it can move.
		if describes && (carrierPlace.Contains(lower) && !placeless || !predicateFollows && (word.Tag == "VBN" || word.Tag == "VBG")) {
			return head, restate(source, prev, rest), opensWithCarrier
		}
		return head, "", 0
	case !hasMain || splitsCoordination(whole, main, first):
		return head, "", 0
	case lower == "so" && (seam == "," || seam == "") && (main.Imperative || instructs(whole)):
		// A so after an instruction states its purpose, and the purpose goes behind "Do this".
		return head, "Do this " + rest, opensWithCarrier
	case describes && !placeless && carrierPlace.Contains(lower) && !strings.HasPrefix(strings.ToLower(rest), "in order"):
		return head, restate(source, prev, rest), opensWithCarrier
	case describes && !predicateFollows && (word.Tag == "VBN" || word.Tag == "VBG") && first+1 < len(whole.Words) && !strings.HasPrefix(whole.Words[first+1].Tag, "NN"):
		return head, restate(source, prev, rest), opensWithCarrier
	case seam == "" && noun && (lower == "that" || lower == "which" || lower == "who") && verbAt(whole, first+1) && (plainPhrase(whole, first+2) || relativeTail(whole, first+2)):
		return head, restateBare(source, prev, strings.TrimLeft(rest[len(word.Text):], " ")), opensWithCarrier
	case seam == "" && noun && !indefinites.Contains(prev.Lower()) && (lower == "that" || lower == "which") && relativeWithSubject(whole, first):
		// "a plugin that the later run took from a cache": the relative clause has a subject of its own.
		pronoun := "one"
		if prev.Tag == "NNS" || prev.Tag == "NNPS" {
			pronoun = "ones"
		}
		return head, restate(source, prev, pronoun+" "+rest), opensWithCarrier
	// "the words after the cut are no clause": a verb after the phrase makes it part of the subject.
	case seam != ":" && carrierAdverbial.Contains(lower) && !standsWithMainClause(rest) && !(noun && seam == "" && predicateFollows && !clauseOpeners.Contains(lower)) &&
		adverbialMoves(whole, first, prev, carrierFor(whole, main) == "This holds") &&
		(!inRelative(whole, first-1) || everyNext(whole, first)) && !coordinatedVerb(whole, first):
		// A reason reads behind "This is", whatever the verb: "This is because X".
		if lower == "because" {
			return head, "This is " + rest, opensWithCarrier
		}
		return head, carrierFor(whole, main) + " " + rest, opensWithCarrier
	case seam == "," && word.Tag == whole.Words[main.Head].Tag && finiteVerbTag(word.Tag) && unicode.IsLower(rune(word.Text[0])) &&
		listsVerbs(source[:c.left]) && !conjunctionBetween(whole, main.Last+1, first) && !subordinatorBetween(whole, main.Last+1, first+1):
		// The rest of a list of verb groups keeps its verbs behind the subject: "It also collapses X, and hides Y".
		subject := mainSubject(source, whole, main, word.Tag)
		if subject == "" {
			return head, "", 0
		}
		return listHead(head, "and", false, whole, main), capitalizeOpening(subject) + " also " + rest, opensWithCarrier
	case seam == "," || seam == ":":
		// "X must ensure that A, and that B" divides with the main clause
		// restated: "X must also ensure that B".
		if seam == "," && lower == "and" && first+1 < len(whole.Words) && whole.Words[first+1].Lower() == "that" && !headOpen(whole, first) {
			if subject := mainSubject(source, whole, main, whole.Words[main.Head].Tag); subject != "" {
				if clause := strings.TrimLeft(rest[len(word.Text):], " "); clause != "" {
					return head, capitalizeOpening(subject) + " " + alsoVerb(source, whole, main) + " " + clause, opensWithCarrier
				}
			}
		}
		// A list that opens after "so" or a subordinator belongs to that clause: "is a thin wrapper, so a hook, a CI job and an editor integration all get".
		if carried, conj, ok := listRest(source, whole, main, c, rest, seam); ok && !subordinatorBetween(whole, main.Last+1, first+1) {
			return listHead(head, conj, oxford(rest), whole, main), carried, opensWithCarrier
		}
		if seam != ":" || lowerIdentifier(firstToken.FindString(rest)) {
			break
		}
		// After a colon, words with a verb of their own are a sentence the parser missed, as with a gerund subject: "keying on it let".
		if (word.Tag == "VBG" || word.Tag == "PRP") && finiteBefore(whole, first, ",") {
			return head, capitalizeOpening(rest), opensWithCarrier
		}
		// Noun phrases after a colon name what the head speaks of: "the words a repair drops, the phrasings it swaps".
		opensNoun := word.Tag == "DT" || word.Tag == "PRP$" || word.Tag == "JJ" || strings.HasPrefix(word.Tag, "NN")
		// One noun phrase after a colon, whose only verbs sit in its relative clauses, defines the head: "a stated count: a number that is true today".
		if opensNoun && !strings.Contains(rest, ",") && finiteBetween(whole, first, len(whole.Words)) && !finiteOutsideRelative(whole, first) {
			return head, "This is " + rest, opensWithCarrier
		}
		// A clause the tagger misread stands as a sentence of its own.
		if opensNoun && hiddenVerb(whole, first) {
			return head, capitalizeOpening(rest), opensWithCarrier
		}
		if opensNoun && !finiteOutsideReduced(whole, first, len(whole.Words)) {
			return head, "This covers " + rest, opensWithCarrier
		}
	}
	return head, "", 0
}
