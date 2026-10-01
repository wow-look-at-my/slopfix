package ste

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
	"github.com/wow-look-at-my/slopfix/trace"
)

// The warning rules. Each reads a pattern that needs a person's judgment to
// repair, so a finding warns and never fails a check.
const (
	IDInstructionLength = "ste/instruction-length"
	IDPassive           = "ste/passive"
	IDNounCluster       = "ste/noun-cluster"
	IDTense             = "ste/tense"
	IDDictionary        = "ste/dictionary"
	IDParagraphLength   = "ste/paragraph-length"
)

// WarningIDs names every rule whose findings are warnings.
var WarningIDs = set.Of(IDInstructionLength, IDPassive, IDNounCluster, IDTense, IDDictionary, IDParagraphLength)

// SeverityWarning marks a finding that informs and does not fail. A finding with no severity is an error.
const SeverityWarning = "warning"

// InstructionWordCap is STE's cap for an instruction.
const InstructionWordCap = 20

// ParagraphSentenceCap is STE's cap on the sentences in a paragraph.
const ParagraphSentenceCap = 6

// NounClusterCap is the most nouns STE lets stand together.
const NounClusterCap = 3

//go:embed dictionary.txt
var dictionaryText string

// dictionary maps a word STE does not approve to the approved words for it.
// A word that is approved in any sense is absent, because a match by spelling
// cannot tell the senses apart.
var dictionary = func() map[string][]string {
	out := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(dictionaryText), "\n") {
		fields := strings.Fields(line)
		out[fields[0]] = fields[1:]
	}
	return out
}()

var (
	beForms = set.Of("am", "is", "are", "was", "were", "be", "been", "being")
	// haveForms open a perfect tense. "will have" is the future perfect.
	haveForms = set.Of("has", "have", "had")
	// approvedIng are the -ing words the STE dictionary approves as an adjective, a pronoun or a preposition.
	approvedIng = set.Of("mating", "missing", "remaining", "something", "during")
	// masks are the words strip writes over data.
	masks = set.Of("CODE", "URL", "ENTITY", "QUOTE")
)

// Warn reports every warning rule the text breaks. Check never answers these,
// because a caller of Check reads any finding as a failure. A list item never
// counts as a long paragraph.
func Warn(text string, line int, listItem bool) []Finding {
	defer trace.Phase("rule/ste-warnings")()
	prose := strip(text)
	var out []Finding
	sentences := 0
	for _, span := range sentenceSpans(prose) {
		text := prose[span[0]:span[1]]
		sentences++
		if n := WordCount(text); n > InstructionWordCap && n <= SentenceWordCap {
			out = append(out, warn(line, IDInstructionLength,
				fmt.Sprintf("over the %d-word cap for an instruction at %d words", InstructionWordCap, n),
				truncate(strings.TrimSpace(text)), "Split it if it tells the reader to do something."))
		}
		off := opaque(text, text)
		out = append(out, verbWarnings(syntax.Parse(text, off), off, line)...)
	}
	for _, word := range wordPattern.FindAllString(prose, -1) {
		// A word in capitals is a name or a mask, and never a dictionary word.
		if word == strings.ToUpper(word) {
			continue
		}
		if alts, banned := dictionary[strings.ToLower(word)]; banned {
			out = append(out, warn(line, IDDictionary, "the STE dictionary does not approve this word", word,
				"Write "+strings.Join(alts, " or ")+"."))
		}
	}
	if sentences > ParagraphSentenceCap && !listItem {
		out = append(out, warn(line, IDParagraphLength,
			fmt.Sprintf("over the %d-sentence cap for a paragraph at %d sentences", ParagraphSentenceCap, sentences),
			"", "Divide the paragraph where its topic changes."))
	}
	return out
}

// verbWarnings reads the tagged words for the passive voice, a complex tense
// and a noun cluster.
func verbWarnings(s *syntax.Sentence, off [][]int, line int) []Finding {
	var out []Finding
	words := s.Words
	nouns := 0
	for i, w := range words {
		// Code, a quotation and a parenthetical are data, so each one ends a cluster.
		data := insideAny(off, w.Start) || masks.Contains(w.Text)
		if strings.HasPrefix(w.Tag, "NN") && !data {
			nouns++
		} else {
			nouns = 0
		}
		if nouns == NounClusterCap+1 {
			out = append(out, warn(line, IDNounCluster, "more than three nouns stand together", s.Span(i-NounClusterCap, i),
				"Break the cluster up with a preposition or a relative clause."))
		}
		lower := w.Lower()
		if !beForms.Contains(lower) && !haveForms.Contains(lower) {
			continue
		}
		next := i + 1
		for next < len(words) && (words[next].Tag == "RB" || words[next].Lower() == "not") {
			next++
		}
		if next >= len(words) {
			continue
		}
		verb := words[next]
		switch {
		case haveForms.Contains(lower) && verb.Tag == "VBN":
			out = append(out, warn(line, IDTense, "STE allows only the simple tenses, and this is a perfect tense",
				s.Span(i, next), "Write the simple past or the simple present."))
		case beForms.Contains(lower) && verb.Tag == "VBG" && !approvedIng.Contains(verb.Lower()) && lower != "being":
			out = append(out, warn(line, IDTense, "STE allows only the simple tenses, and this is a progressive tense",
				s.Span(i, next), "Write the simple present."))
		case beForms.Contains(lower) && verb.Tag == "VBN":
			out = append(out, warn(line, IDPassive, "STE prefers the active voice", s.Span(i, next),
				"Name the actor as the subject."))
		}
	}
	return out
}

func warn(line int, id, rule, detail, fix string) Finding {
	return Finding{Line: line, ID: id, Rule: rule, Detail: detail, Fix: fix, Severity: SeverityWarning}
}
