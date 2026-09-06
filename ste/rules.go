// Package ste checks prose against ASD-STE100, Simplified Technical English.
//
// STE is a controlled language: each approved word carries a single meaning and
// a single part of speech, and its rules keep every sentence to a single
// reading. It suits code prose for the reason it suits a maintenance manual. The
// reader is about to change the thing described, and nobody is there to ask.
package ste

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
)

// Finding is a rule the line breaks, and how to repair it.
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

// SentenceWordCap is STE's limit for a descriptive sentence. An instruction has
// a tighter limit, which this checker does not try to tell apart.
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

const (
	// clauseSubject opens a clause.
	clauseSubject = `it|they|he|she|we|you|this|that|these|those|nothing|everything|nobody|the|a|an`
	// finiteVerb marks a clause. A participle and an infinitive do not, which
	// keeps an ordinary phrase off the list.
	finiteVerb = `is|are|was|were|has|have|had|does|do|did|can|cannot|must|will|makes|make|means|` +
		`gives|gets|goes|comes|stays|keeps|needs|wants|says|reads|holds|owns|carries|fires|runs|` +
		`works|costs|counts|leaves|starts|stops|takes|tells|turns|lives|looks|sits|sends|renders|` +
		`answers|reports|names|breaks|ends|fits|fails|passes|applies|returns|sets|adds|drops|` +
		`moves|calls|opens|closes|splits|joins|emits|writes|prints|expands`
)

var (
	// wordPattern keeps an internal apostrophe, so a contraction stays intact.
	wordPattern = regexp.MustCompile(`[A-Za-z]+(?:'[A-Za-z]+)?`)
	// codeSpan matches an inline code span, whose contents are data.
	codeSpan = regexp.MustCompile("`[^`]*`")
	// linkTarget matches a markdown link's URL, which is not prose.
	linkTarget = regexp.MustCompile(`\]\([^)]*\)`)
	// entity ENDS IN A SEMICOLON, which unmasked prose reports as its own.
	entity = regexp.MustCompile(`&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[a-zA-Z][a-zA-Z0-9]{1,31});`)
	// parenthetical counts as a single word, which STE says and a citation needs.
	parenthetical = regexp.MustCompile(`\([^()]*\)`)
	// commaSplice wants a subject and a finite verb after the comma. The
	// conjunction is optional, because a bare comma splices too.
	commaSplice = regexp.MustCompile(
		`(?i),\s+((?:and|but|so|yet|then)\s+)?(?:` + clauseSubject +
			`)\s+(?:\w+\s+){0,2}?(?:not\s+)?(?:` + finiteVerb + `)\b`)
	finiteVerbRe = regexp.MustCompile(`(?i)\b(?:` + finiteVerb + `)\b`)
	// subordinator opens a dependent clause, which a comma may join.
	subordinator = regexp.MustCompile(`(?i)^(?:(?:and|but|so|or|yet)\s+)?` +
		`(?:if|when|whenever|where|wherever|while|because|although|though|unless|since|after|` +
		`before|until|once|whether|provided|assuming|given|for)\b`)
	// countPattern finds a number that counts items. The count stays true until
	// somebody changes the set, and nothing corrects it then.
	countPattern = regexp.MustCompile(`(?i)\b(two|three|four|five|six|seven|eight|nine|ten|` +
		`eleven|twelve|[0-9]+)\s+(?:[a-z-]+\s+){0,2}?([a-z]+s)\b`)
	// fileOrSection matches a lower-case sentence opener: a file name, or a
	// section reference. Demanding a capital welds it onto its predecessor.
	fileOrSection = regexp.MustCompile(`^(?:[A-Za-z0-9_.-]+\.` +
		`(?:md|go|ts|tsx|js|jsx|yml|yaml|json|sh|txt|html|css|rs|py|toml|sql|proto)|§|¶)`)
)

// abbreviations end in a period that never ends a sentence.
var abbreviations = set.Of[string](
	"e.g.", "i.e.", "etc.", "vs.", "cf.", "al.", "fig.", "no.", "approx.", "ca.", "resp.",
)

// units are measured, not counted. A budget in characters names a size, and
// nobody revisits it when a list grows.
var units = set.Of[string](
	"bits", "bytes", "kilobytes", "megabytes", "gigabytes", "characters", "chars", "runes",
	"words", "lines", "columns", "rows", "spaces", "digits", "seconds", "minutes", "hours",
	"days", "weeks", "months", "years", "milliseconds", "microseconds", "nanoseconds",
	"pixels", "points", "percent", "times", "levels", "degrees",
)

// Check reports every rule the text breaks. The text is a prose block already
// joined to a single line, and line is where it starts in the source.
func Check(text string, line int) []Finding {
	prose := strip(text)
	var out []Finding
	out = append(out, checkWords(prose, line)...)
	out = append(out, checkSemicolons(prose, line)...)
	out = append(out, checkSentences(prose, line)...)
	out = append(out, checkSplices(prose, line)...)
	out = append(out, checkCounts(prose, line)...)
	return out
}

// strip removes the spans that are data rather than prose: inline code, a
// link's target, and an HTML entity. A semicolon inside any of them is not a
// sentence joiner.
func strip(text string) string {
	text = codeSpan.ReplaceAllString(text, " CODE ")
	text = linkTarget.ReplaceAllString(text, "](URL)")
	return entity.ReplaceAllString(text, " ENTITY ")
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
	for _, sentence := range Sentences(prose) {
		words := WordCount(sentence)
		if words <= SentenceWordCap {
			continue
		}
		out = append(out, Finding{
			line,
			fmt.Sprintf("over the %d-word sentence cap at %d words", SentenceWordCap, words),
			truncate(strings.TrimSpace(sentence)),
			"Split it into shorter sentences.",
		})
	}
	return out
}

// checkSplices finds a comma doing a period's job.
//
// A clause must follow the comma: a subject, then a finite verb. That test
// leaves a list and an Oxford comma alone, because neither carries a verb.
//
// The bare form, with no conjunction, needs a further test. An introductory
// phrase reads the same way, so there the words BEFORE the comma must carry a
// finite verb too. A conjunction joins equals, and says as much by itself.
func checkSplices(prose string, line int) []Finding {
	var out []Finding
	for _, loc := range commaSplice.FindAllStringSubmatchIndex(prose, -1) {
		bare := loc[2] < 0
		if bare && !isClause(clauseBefore(prose, loc[0])) {
			continue
		}
		out = append(out, Finding{
			line,
			"a comma joining two clauses is the semicolon STE bans, spelled differently",
			strings.TrimSpace(prose[loc[0]:loc[1]]),
			"Write a period in place of the comma and capitalize the next word.",
		})
	}
	return out
}

// checkCounts finds a stated count of items. The reader trusts the number long
// after somebody adds the item that makes it wrong.
func checkCounts(prose string, line int) []Finding {
	var out []Finding
	for _, loc := range countPattern.FindAllStringSubmatchIndex(prose, -1) {
		noun := strings.ToLower(prose[loc[4]:loc[5]])
		if units.Contains(noun) || inExpression(prose, loc[2]) {
			continue
		}
		out = append(out, Finding{
			line,
			"a stated count goes stale when the set changes",
			strings.TrimSpace(prose[loc[0]:loc[1]]),
			"Describe what is there and let the reader count.",
		})
	}
	return out
}

// inExpression reports a number that is arithmetic rather than a count. The
// digits in an expression or a range name no set of items.
func inExpression(prose string, start int) bool {
	if start == 0 {
		return false
	}
	before := []rune(prose[:start])
	prev := before[len(before)-1]
	switch prev {
	case '-', '−', '+', '/', '*', '=', '.', ',', '_':
		return true
	}
	return unicode.IsDigit(prev)
}

// clauseBefore returns the words from the end of the previous sentence up to
// the comma at idx.
func clauseBefore(prose string, idx int) string {
	before := prose[:idx]
	if cut := strings.LastIndexAny(before, ".!?"); cut >= 0 {
		before = before[cut+1:]
	}
	return before
}

// isClause reports whether the text already stands alone, which is what makes
// the words after the comma a further clause rather than the sentence's only.
func isClause(before string) bool {
	words := strings.TrimSpace(before)
	if subordinator.MatchString(words) {
		return false
	}
	return finiteVerbRe.MatchString(words) && len(strings.Fields(words)) >= 3
}

// Sentences splits prose into sentences.
//
// A period ends a sentence only when what follows opens the next. That rules
// out "e.g. the lexer" and the "$(...)" of a shell example, which a plain
// period-and-space split cuts apart. An oversized sentence then reads as short
// pieces and escapes the cap.
//
// A lower-case opener still starts a sentence when it is a file name or a
// section mark. Demanding a capital there is the mirror defect, and it hides
// the length of everything it welds together.
func Sentences(text string) []string {
	var out []string
	runes := []rune(text)
	start := 0
	for i := 0; i < len(runes); i++ {
		if !terminator(runes[i]) {
			continue
		}
		end := i
		for end+1 < len(runes) && terminator(runes[end+1]) {
			end++
		}
		gap := end + 1
		if gap >= len(runes) || !unicode.IsSpace(runes[gap]) {
			i = end
			continue
		}
		next := gap
		for next < len(runes) && unicode.IsSpace(runes[next]) {
			next++
		}
		if next >= len(runes) || !opensSentence(runes[next:]) || endsWithAbbreviation(runes[start:end+1]) {
			i = end
			continue
		}
		out = append(out, strings.TrimSpace(string(runes[start:end+1])))
		start = next
		i = next - 1
	}
	if rest := strings.TrimSpace(string(runes[start:])); rest != "" {
		out = append(out, rest)
	}
	return out
}

func terminator(r rune) bool {
	return r == '.' || r == '!' || r == '?'
}

// opensSentence reports whether the text starts a new sentence. A capital, a
// digit and an opening delimiter each do, and so does a lower-case file name.
func opensSentence(rest []rune) bool {
	switch first := rest[0]; {
	case unicode.IsUpper(first), unicode.IsDigit(first):
		return true
	case strings.ContainsRune("(`'\"*_[§¶“‘", first):
		return true
	}
	return fileOrSection.MatchString(string(rest))
}

func endsWithAbbreviation(sentence []rune) bool {
	fields := strings.Fields(string(sentence))
	if len(fields) == 0 {
		return false
	}
	return abbreviations.Contains(strings.ToLower(fields[len(fields)-1]))
}

// WordCount counts the words in a sentence. Text in parentheses counts as a
// single word, whatever it holds, which is what STE says.
func WordCount(sentence string) int {
	collapsed := parenthetical.ReplaceAllString(sentence, " x ")
	return len(wordPattern.FindAllString(collapsed, -1))
}

func truncate(s string) string {
	const limit = 60
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
