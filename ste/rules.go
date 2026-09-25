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
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
	"github.com/wow-look-at-my/slopfix/trace"
)

// The rule IDs. A report prints the ID that found the text, and the same ID
// selects that rule on the command line.
const (
	IDContraction = "ste/contraction"
	IDModal       = "ste/modal"
	IDSemicolon   = "ste/semicolon"
	IDSentenceCap = "ste/sentence-length"
	IDCommaSplice = "ste/comma-splice"
	IDStaleCount  = "ste/count"
)

// AllIDs names every rule this package reports. A set, because every consumer
// asks whether a name is in it rather than reading it in order.
var AllIDs = set.Of(
	IDContraction, IDModal, IDSemicolon, IDSentenceCap, IDCommaSplice, IDStaleCount,
	IDPostdeterminer,
)

// Finding is a rule the line breaks, and how to repair it.
type Finding struct {
	Line int
	// EndLine is the last line covered, unset when the finding sits on Line alone.
	EndLine int
	// ID names the rule, and selects it on the command line.
	ID   string
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
	return fmt.Sprintf("%d: [%s] %s%s. %s", f.Line, f.ID, f.Rule, detail, f.Fix)
}

// SentenceWordCap is STE's cap for a descriptive sentence.
const SentenceWordCap = 25

// steTable is what rules/ says for="ste": the banned forms, their replacements, and the word classes the clause shapes
var steTable = table.MustLoad(rules.FS, "ste")

// irregulars maps a contraction whose ending does not spell its expansion.
var irregulars, modals = swaps()

func swaps() (map[string]string, map[string]string) {
	odd, hedges := map[string]string{}, map[string]string{}
	for _, r := range steTable.Rewrites {
		if r.Where == "modal" {
			hedges[strings.ToLower(r.From)] = r.To
			continue
		}
		odd[strings.ToLower(r.From)] = r.To
	}
	return odd, hedges
}

func Expand(word string) (string, bool) {
	if full, odd := irregulars[strings.ToLower(word)]; odd {
		return full, true
	}
	for _, p := range steTable.Patterns {
		if p.Where == "contraction" && p.Matches(word) {
			return p.Replace(word), true
		}
	}
	return "", false
}

// alternation writes a named class as a regexp branch, in the order the table carries the words.
func alternation(name string) string { return strings.Join(wordsOf(name), "|") }

var (
	// clauseSubject opens a clause.
	clauseSubject = alternation("clause-subject")
	// finiteVerb marks a clause. A participle and an infinitive do not, which keeps an ordinary phrase off the list.
	finiteVerb = alternation("finite-verb")
	// spliceConjunction may stand in front of the spliced clause, and subordinatorConjunction in front of a subordinator.
	spliceConjunction       = alternation("splice-conjunction")
	subordinatorConjunction = alternation("subordinator-conjunction")
	// subordinators open a dependent clause, which a comma may join.
	subordinators = alternation("subordinator")
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
		`(?i),\s+((?:` + spliceConjunction + `)\s+)?(?:` + clauseSubject +
			`)\s+(?:\w+\s+){0,2}?(?:not\s+)?(?:` + finiteVerb + `)\b`)
	finiteVerbRe = regexp.MustCompile(`(?i)\b(?:` + finiteVerb + `)\b`)
	// subordinator opens a dependent clause, which a comma may join.
	subordinator = regexp.MustCompile(`(?i)^(?:(?:` + subordinatorConjunction + `)\s+)?` +
		`(?:` + subordinators + `)\b`)
	// fileOrSection matches a lower-case sentence opener: a file name, or a
	// section reference. Demanding a capital welds it onto its predecessor.
	fileOrSection = regexp.MustCompile(`^(?:[A-Za-z0-9_.-]+\.` +
		`(?:md|go|ts|tsx|js|jsx|yml|yaml|json|sh|txt|html|css|rs|py|toml|sql|proto)|§|¶)`)
)

// abbreviations end in a period that never ends a sentence.
var abbreviations = set.Of(wordsOf("abbreviation")...)

// wordsOf answers a named class's word list.
func wordsOf(name string) []string {
	for _, c := range steTable.Classes {
		if c.Name == name {
			return c.Words
		}
	}
	panic("ste: rules/ names no class " + name)
}

// Check reports every rule the text breaks. The text is a prose block already
// joined to a single line, and line is where it starts in the source.
func Check(text string, line int) []Finding {
	prose := strip(text)
	var out []Finding
	for _, rule := range proseRules {
		out = append(out, rule.run(prose, line)...)
	}
	return out
}

// proseRule is a single rule under the phase name a timing run prints for it.
type proseRule struct {
	phase string
	check func(prose string, line int) []Finding
}

// run reads the prose under this rule, inside the rule's own timing phase.
func (r proseRule) run(prose string, line int) []Finding {
	defer trace.Phase(r.phase)()
	return r.check(prose, line)
}

// proseRules are the prose rules in the order a report prints them. A phase
// name rather than a rule ID, because the word rule reports IDs.
var proseRules = []proseRule{
	{"rule/ste-words", checkWords},
	{"rule/ste-semicolon", checkSemicolons},
	{"rule/ste-sentence-length", checkSentences},
	{"rule/ste-comma-splice", checkSplices},
	{"rule/ste-count", checkCounts},
	{"rule/ste-postdeterminer", checkPostdeterminers},
}

// strip removes the spans that are data rather than prose: inline code, a
// link's target, and an HTML entity. A semicolon inside any of them is not a
// sentence joiner.
func strip(text string) string {
	defer trace.Phase("rule/ste-strip")()
	text = codeSpan.ReplaceAllString(text, " CODE ")
	text = linkTarget.ReplaceAllString(text, "](URL)")
	return entity.ReplaceAllString(text, " ENTITY ")
}

func checkWords(prose string, line int) []Finding {
	var out []Finding
	for _, word := range wordPattern.FindAllString(prose, -1) {
		lower := strings.ToLower(word)
		if fix, banned := Expand(word); banned {
			out = append(out, Finding{Line: line, ID: IDContraction, Rule: "STE bans contractions", Detail: word, Fix: "Write " + fix + "."})
		}
		if fix, banned := modals[lower]; banned {
			out = append(out, Finding{Line: line, ID: IDModal, Rule: "STE bans this modal", Detail: word, Fix: "Write " + fix + ", or rewrite the sentence."})
		}
	}
	return out
}

func checkSemicolons(prose string, line int) []Finding {
	if !strings.Contains(prose, ";") {
		return nil
	}
	return []Finding{{Line: line, ID: IDSemicolon, Rule: "STE bans the semicolon", Detail: ";", Fix: "Write a period and start a new sentence."}}
}

func checkSentences(prose string, line int) []Finding {
	var out []Finding
	for _, sentence := range Sentences(prose) {
		words := WordCount(sentence)
		if words <= SentenceWordCap {
			continue
		}
		out = append(out, Finding{
			Line:   line,
			ID:     IDSentenceCap,
			Rule:   fmt.Sprintf("over the %d-word sentence cap at %d words", SentenceWordCap, words),
			Detail: truncate(strings.TrimSpace(sentence)),
			Fix:    "Split it into shorter sentences.",
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
		if !spliced(prose, loc) {
			continue
		}
		out = append(out, Finding{
			Line:   line,
			ID:     IDCommaSplice,
			Rule:   "a comma joining two clauses is the semicolon STE bans, spelled differently",
			Detail: strings.TrimSpace(prose[loc[0]:loc[1]]),
			Fix:    "Write a period in place of the comma and capitalize the next word.",
		})
	}
	return out
}

// checkCounts finds a stated count of items. The reader trusts the number long
// after somebody adds the item that makes it wrong.
// This rule's own spelling of a count lives in cardinal as the Gate substrate. The document rule and the comment rule read
// the same package with their own policies, so they cannot drift apart.
func checkCounts(prose string, line int) []Finding {
	var out []Finding
	for _, found := range cardinal.Find(prose, cardinal.Gate) {
		out = append(out, Finding{
			Line:   line,
			ID:     IDStaleCount,
			Rule:   "a stated count goes stale when the set changes",
			Detail: strings.TrimSpace(found.Text),
			Fix:    "Describe what is there and let the reader count.",
		})
	}
	return out
}

// clauseBefore returns the words from the end of the previous sentence up to
// the comma at idx.
func clauseBefore(prose string, idx int) string {
	before := prose[:idx]
	if cut := strings.LastIndexAny(before, ".!?:;—"); cut >= 0 {
		_, width := utf8.DecodeRuneInString(before[cut:])
		before = before[cut+width:]
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
