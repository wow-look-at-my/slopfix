package ste

import (
	"fmt"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
	"github.com/wow-look-at-my/slopfix/trace"
)

// The warning rules. A finding warns and never fails a check, and `slopfix
// fix` rewrites each.
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

// dictionary maps a word STE does not approve to the approved words for it.
// rules/ste-dictionary.xml holds it, a word and its replacements on each line.
var dictionary = func() map[string][]string {
	out := map[string][]string{}
	for _, line := range steTable.List("dictionary") {
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
		off := opaque(text, text)
		s := syntax.Parse(text, off)
		if n := WordCount(text); n > InstructionWordCap && n <= SentenceWordCap && isInstruction(s) {
			out = append(out, warn(line, IDInstructionLength,
				fmt.Sprintf("over the %d-word cap for an instruction at %d words", InstructionWordCap, n),
				truncate(strings.TrimSpace(text)), "Split it into shorter instructions. `slopfix fix` does this."))
		}
		out = append(out, clauseWarnings(s, text, off, line)...)
	}
	for _, loc := range wordPattern.FindAllStringIndex(prose, -1) {
		word := prose[loc[0]:loc[1]]
		// A word in capitals is a name or a mask, and never a dictionary word.
		if word == strings.ToUpper(word) {
			continue
		}
		if approved, banned := plainSwap(prose, loc); banned {
			out = append(out, warn(line, IDDictionary, "the STE dictionary does not approve this word", word,
				"Write "+approved+"."))
		}
	}
	if sentences > ParagraphSentenceCap && !listItem {
		out = append(out, warn(line, IDParagraphLength,
			fmt.Sprintf("over the %d-sentence cap for a paragraph at %d sentences", ParagraphSentenceCap, sentences),
			"", "Divide the paragraph where its topic changes."))
	}
	return out
}

// clauseWarnings reports the passive voice, a complex tense and a noun
// cluster in a sentence, each where its repair rewrites the sentence. The
// repair reads the same parse. A finding here is a rewrite `slopfix fix`
// makes, and a form no rewrite can say otherwise is no finding.
func clauseWarnings(s *syntax.Sentence, source string, off [][]int, line int) []Finding {
	var out []Finding
	if _, ok := rewritePassive(s, source); ok {
		i, j := auxiliary(s, 1, beForms)
		out = append(out, warn(line, IDPassive, "STE prefers the active voice", s.Span(i, j),
			"Name the actor as the subject. `slopfix fix` does this."))
	}
	if _, ok := rewriteTense(s, source); ok {
		i, j, perfect := tenseAt(s)
		rule := "STE allows only the simple tenses, and this is a progressive tense"
		if perfect {
			rule = "STE allows only the simple tenses, and this is a perfect tense"
		}
		out = append(out, warn(line, IDTense, rule, s.Span(i, j), "Write the simple tense. `slopfix fix` does this."))
	}
	if first, last, ok := clusterAt(s, off); ok {
		out = append(out, warn(line, IDNounCluster, "more than three nouns stand together", s.Span(first, last),
			"Break the cluster up with a preposition. `slopfix fix` does this."))
	}
	return out
}

// isInstruction reports a sentence that tells the reader to act: it opens on
// the base form of a verb.
func isInstruction(s *syntax.Sentence) bool {
	for _, w := range s.Words {
		if punctuationTag(w.Tag) || w.Tag == "RB" {
			continue
		}
		return w.Tag == "VB" || w.Tag == "VBP"
	}
	return false
}

func warn(line int, id, rule, detail, fix string) Finding {
	return Finding{Line: line, ID: id, Rule: rule, Detail: detail, Fix: fix, Severity: SeverityWarning}
}
