// Package ste checks prose against ASD-STE100, Simplified Technical English.
//
// STE is a controlled language: an approved word has one meaning and one part
// of speech, and a rule set keeps every sentence to a single reading. It suits
// code prose for the reason it suits a maintenance manual. The reader is about
// to change the thing being described, and nobody is there to ask.
package ste

import (
	"fmt"
	"regexp"
	"strings"
)

// Finding is one rule a line breaks, and how to repair it.
type Finding struct {
	Line int
	Rule string
	// Detail quotes the offending text.
	Detail string
	// Fix names the repair, in the imperative.
	Fix string
}

func (f Finding) String() string {
	detail := ""
	if f.Detail != "" {
		detail = fmt.Sprintf(" %q", f.Detail)
	}
	return fmt.Sprintf("%d: %s%s. %s", f.Line, f.Rule, detail, f.Fix)
}

// SentenceWordCap is STE's limit for a descriptive sentence. An instruction is
// held to a shorter one, which this checker does not try to tell apart.
const SentenceWordCap = 25

// contractions maps every banned form to the words STE writes instead.
var contractions = map[string]string{
	"can't": "cannot", "won't": "will not", "don't": "do not", "doesn't": "does not",
	"didn't": "did not", "isn't": "is not", "aren't": "are not", "wasn't": "was not",
	"weren't": "were not", "wouldn't": "will not", "shouldn't": "must not",
	"couldn't": "cannot", "mustn't": "must not", "hasn't": "has not",
	"haven't": "have not", "hadn't": "had not", "it's": "it is", "that's": "that is",
	"there's": "there is", "here's": "here is", "let's": "let us", "we're": "we are",
	"they're": "they are", "you're": "you are", "i'm": "I am", "i've": "I have",
	"we've": "we have", "they've": "they have", "you've": "you have",
	"i'll": "I will", "we'll": "we will", "they'll": "they will", "you'll": "you will",
	"it'll": "it will", "what's": "what is", "who's": "who is",
}

// modals maps each banned modal to the word STE approves for that sense.
var modals = map[string]string{
	"should": "must", "shall": "must", "could": "can", "might": "can", "would": "will",
}

// spliceConjunctions open a clause that a comma must not join to the one before
// it. Each is a full sentence's worth of new subject and verb.
var spliceConjunctions = []string{"so", "then", "therefore", "thus", "however"}

var (
	// wordPattern finds a word, keeping an internal apostrophe so a contraction
	// stays one token.
	wordPattern = regexp.MustCompile(`[A-Za-z]+(?:'[A-Za-z]+)?`)
	// sentenceEnd splits on terminal punctuation followed by a space.
	sentenceEnd = regexp.MustCompile(`[.!?]+\s+`)
	// codeSpan matches an inline code span, whose contents are data.
	codeSpan = regexp.MustCompile("`[^`]*`")
	// linkTarget matches a markdown link's URL, which is not prose.
	linkTarget = regexp.MustCompile(`\]\([^)]*\)`)
)

// Check reports every rule the text breaks. The text is one prose block already
// joined to a single line, and line is where it starts in the source.
func Check(text string, line int) []Finding {
	prose := strip(text)
	var out []Finding
	out = append(out, checkWords(prose, line)...)
	out = append(out, checkSemicolons(prose, line)...)
	out = append(out, checkSentences(prose, line)...)
	out = append(out, checkSplices(prose, line)...)
	return out
}

// strip removes the spans that are data rather than prose: inline code, and a
// link's target. A semicolon inside either is not a sentence joiner.
func strip(text string) string {
	text = codeSpan.ReplaceAllString(text, " CODE ")
	return linkTarget.ReplaceAllString(text, "](URL)")
}

func checkWords(prose string, line int) []Finding {
	var out []Finding
	for _, word := range wordPattern.FindAllString(prose, -1) {
		lower := strings.ToLower(word)
		if fix, banned := contractions[lower]; banned {
			out = append(out, Finding{line, "STE bans contractions", word, "Write " + fix + "."})
		}
		if fix, banned := modals[lower]; banned {
			out = append(out, Finding{line, "STE bans this modal", word, "Write " + fix + ", or rewrite the sentence."})
		}
	}
	return out
}

func checkSemicolons(prose string, line int) []Finding {
	if !strings.Contains(prose, ";") {
		return nil
	}
	return []Finding{{line, "STE bans the semicolon", ";", "Write a period and start a new sentence."}}
}

func checkSentences(prose string, line int) []Finding {
	var out []Finding
	for _, sentence := range sentenceEnd.Split(prose, -1) {
		words := wordPattern.FindAllString(sentence, -1)
		if len(words) <= SentenceWordCap {
			continue
		}
		out = append(out, Finding{
			line,
			fmt.Sprintf("over the %d-word sentence cap at %d words", SentenceWordCap, len(words)),
			truncate(strings.TrimSpace(sentence)),
			"Split it into shorter sentences.",
		})
	}
	return out
}

// checkSplices finds a comma doing a period's job: joining two clauses that each
// stand alone. It looks for the conjunctions that open such a clause, which is
// where the pattern is unambiguous.
func checkSplices(prose string, line int) []Finding {
	var out []Finding
	lower := strings.ToLower(prose)
	for _, conj := range spliceConjunctions {
		needle := ", " + conj + " "
		if idx := strings.Index(lower, needle); idx >= 0 {
			out = append(out, Finding{
				line,
				"a comma joining two clauses is the semicolon STE bans, spelled differently",
				strings.TrimSpace(prose[idx:min(idx+40, len(prose))]),
				"Write a period in place of the comma and capitalize the next word.",
			})
		}
	}
	return out
}

func truncate(s string) string {
	const limit = 60
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
