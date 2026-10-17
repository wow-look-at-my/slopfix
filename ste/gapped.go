package ste

import (
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// restStands reports a rest a division may open as a sentence: it holds a
// main clause, or it opens a list.
func restStands(rest string) bool {
	return opensList(rest) || standsAsSentence(rest)
}

// gappedDivision divides a sentence whose main clause is followed by a list of
// gapped items, as in "This happens after X, A after Y, and B after Z". Each
// item leaves out the verb the main clause states, so no item is a sentence of
// its own. Each therefore takes "The same holds for" in place of the verb.
func gappedDivision(source string, whole *syntax.Sentence, limit int) (string, bool) {
	var items [][2]int
	for k, w := range whole.Words {
		if w.Text != "," {
			continue
		}
		j := k + 1
		if j < len(whole.Words) && whole.Words[j].Lower() == "and" {
			j++
		}
		if j < len(whole.Words) && opensFragment(whole.Words[j]) {
			items = append(items, [2]int{k, j})
		}
	}
	// A gapped list holds at least items, the last behind "and". A single item after a comma is an appositive, which names the noun before it.
	if len(items) < 2 || whole.Words[items[len(items)-1][0]+1].Lower() != "and" {
		return source, false
	}
	head := closeHead(source[:whole.Words[items[0][0]].Start])
	if !StandsAlone(head) {
		return source, false
	}
	// Each item repeats the preposition that closes the main clause, in place of its verb: "happens after X, A after Y".
	prep := ""
	for k := items[0][0] - 1; k >= 0 && prep == ""; k-- {
		if w := whole.Words[k]; w.Tag == "IN" || w.Tag == "TO" {
			prep = w.Lower()
		}
	}
	for n, item := range items {
		end := len(whole.Words)
		if n+1 < len(items) {
			end = items[n+1][0]
		}
		if prep == "" || !slices.ContainsFunc(whole.Words[item[1]:end], func(w syntax.Word) bool { return w.Lower() == prep }) {
			return source, false
		}
	}
	out := head
	for n, item := range items {
		end := len(source)
		if n+1 < len(items) {
			end = whole.Words[items[n+1][0]].Start
		}
		text := strings.TrimRight(source[whole.Words[item[1]].Start:end], " .,;")
		out += " The same holds for " + text + "."
	}
	if overCap(out, limit) >= overCap(source, limit) {
		return source, false
	}
	return out, true
}

// insideCompound reports a cut between a noun and the noun or bare verb form
// after it with no mark between them, as in "no visibility | test".
func insideCompound(source string, whole *syntax.Sentence, c forceCut) bool {
	return cutsNounPhrase(source, whole, c.left, c.right)
}

// cutsNounPhrase reports a cut from left to right, with no mark between the
// halves, that lands inside a noun phrase. A determiner or a possessive needs
// the words after it. An adjective, a participle, a numeral or a noun before
// a noun modifies that noun: "a hosted | Linux label". A bare verb form after
// a noun continues it: "no visibility | test".
func cutsNounPhrase(source string, whole *syntax.Sentence, left, right int) bool {
	if strings.TrimSpace(source[left:right]) != "" || seamBefore(source, left) != "" {
		return false
	}
	n := wordsBefore(whole, right)
	if n < 1 || n >= len(whole.Words) {
		return false
	}
	before, after := whole.Words[n-1].Tag, whole.Words[n].Tag
	// "falls | back": a particle completes the verb before it.
	if strings.HasPrefix(before, "VB") && (after == "RP" || phraseEndParticle.Contains(whole.Words[n].Lower())) {
		return true
	}
	switch {
	case before == "DT" || before == "PRP$" || before == "POS":
		return true
	case modifiesNoun.Contains(before) || strings.HasPrefix(before, "NN"):
		if strings.HasPrefix(after, "NN") {
			return true
		}
	}
	return strings.HasPrefix(before, "NN") && (after == "VB" || after == "VBP")
}

// objectClauseOpeners open a clause that is the object of a verb: "reads whether the job runs docker".
var objectClauseOpeners = set.Of("that", "whether", "what", "how", "why", "which", "who", "where", "when")

// modifiesNoun is the tags of a word that modifies the noun after it.
var modifiesNoun = set.Of("JJ", "JJR", "JJS", "VBN", "VBG", "CD")
