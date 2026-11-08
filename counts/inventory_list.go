// inventory_list.go holds the inventory list code.

package counts

import (
	"regexp"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/syntax"
	"github.com/wow-look-at-my/slopfix/table"
)

// IDList names the enumeration rule: prose that lists the members of a set the code defines.
const IDList = "counts/inventory-list"

const (
	// MinMembers is the least number of elements that hold a member and make a list an enumeration.
	MinMembers = 3
	// MinFramedMembers is the least number of elements a list behind a complete-set frame needs.
	MinFramedMembers = 2
)

func init() {
	fixer.Register(fixer.Spec{
		Label: IDList, Families: []string{"counts"}, Rules: []string{IDList},
		Files: []fixer.Kind{fixer.Document}, Place: 19,
		Repair: func(f *fixer.File) { f.ApplyThrough(ListGate, ListEdits(f.Text())) },
	})
}

// listTable is what rules/ says for="inventory-list".
var listTable = table.MustLoad(rules.FS, "inventory-list")

var (
	// listSplit divides a stretch of a sentence at each joiner and lead-in.
	listSplit = splitter(false)
	// boundarySplit also divides at a parenthesis and a colon, which close a list inside a sentence.
	boundarySplit = splitter(true)
	nameShapes    = shapes(listTable.List("name-shape"))
	linkTarget    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	tokenPattern  = regexp.MustCompile("`[^`]*`|\\[[^\\]]*\\]\\([^)\\s]*\\)|\\S+")
)

// splitter builds the pattern that divides a list into its elements.
func splitter(boundaries bool) *regexp.Regexp {
	var alts []string
	for _, class := range []string{"joiner", "lead-in"} {
		for _, w := range listTable.WordsOf(class) {
			phrase := strings.ReplaceAll(regexp.QuoteMeta(w), "_", `\s+`)
			if isLetter(w[0]) {
				phrase = `\b` + phrase
			}
			if isLetter(w[len(w)-1]) {
				phrase += `\b`
			}
			alts = append(alts, phrase)
		}
	}
	if boundaries {
		alts = append(alts, `[:()]`)
	}
	return regexp.MustCompile(`(?i)` + strings.Join(alts, "|"))
}

func shapes(patterns []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

// IsMember reports whether a token names something the code holds. This
// covers a code span, a link into the repository, or a token of the
// name-shape class.
func IsMember(token string) bool {
	if strings.HasPrefix(token, "`") && strings.HasSuffix(token, "`") && len(token) > 2 {
		return true
	}
	if m := linkTarget.FindStringSubmatch(token); m != nil {
		return inRepository(m[1])
	}
	word := strings.Trim(token, ".,;:!?()[]\"'*_")
	if word == "" || strings.Contains(word, "://") {
		return false
	}
	if listTable.Lexicon().Is(word, "name-shape") {
		return true
	}
	for _, re := range nameShapes {
		if re.MatchString(word) {
			return true
		}
	}
	return false
}

// inRepository reports a link target that names a path or a symbol of the repository.
func inRepository(target string) bool {
	return target != "" && !strings.Contains(target, "://") && !strings.HasPrefix(target, "#") && !strings.HasPrefix(target, "mailto:")
}

// holdsMember reports whether a stretch of prose holds a member.
func holdsMember(text string) bool {
	for _, tok := range tokenPattern.FindAllString(text, -1) {
		if IsMember(tok) {
			return true
		}
	}
	return false
}

// firstMember answers the span of the earliest member in text.
func firstMember(text string) (int, int, bool) {
	for _, loc := range tokenPattern.FindAllStringIndex(text, -1) {
		if IsMember(text[loc[0]:loc[1]]) {
			return loc[0], loc[1], true
		}
	}
	return 0, 0, false
}

// element is a phrase of a list, as a byte span into its sentence.
type element struct {
	start, end int
	member     bool
}

// elementsOf divides text[from:to] at each match of split. masked hides what a
// code span or a quotation holds, so a comma inside one divides nothing.
func elementsOf(text, masked string, from, to int, split *regexp.Regexp) []element {
	var out []element
	add := func(a, b int) {
		piece := text[a:b]
		if strings.Trim(piece, " \t\n.,;:!?") == "" {
			return
		}
		out = append(out, element{start: a, end: b, member: holdsMember(piece)})
	}
	at := from
	for _, loc := range split.FindAllStringIndex(masked[from:to], -1) {
		add(at, from+loc[0])
		at = from + loc[1]
	}
	add(at, to)
	return out
}

// listPlace is where an element of a run may sit.
type listPlace struct {
	// head: the element ends on a member, so it can open a run.
	head bool
	// inner: the element holds a member and no finite verb, so it can sit inside a run.
	inner bool
	// tail: the element opens on a member, so it can close a run.
	tail bool
}

// listDeterminers may stand before the member that opens an element.
var listDeterminers = set.Of("a", "an", "the", "each", "every", "this", "these", "those", "any")

// placeOf judges where an element of text may sit in a run. s is the parse of
// the whole sentence.
func placeOf(text string, s *syntax.Sentence, e element) listPlace {
	if !e.member {
		return listPlace{}
	}
	tokens := tokenPattern.FindAllString(text[e.start:e.end], -1)
	for len(tokens) > 0 && strings.Trim(tokens[len(tokens)-1], ".,;:!?") == "" {
		tokens = tokens[:len(tokens)-1]
	}
	if len(tokens) == 0 {
		return listPlace{}
	}
	first := 0
	for first < len(tokens)-1 && listDeterminers.Contains(strings.ToLower(tokens[first])) {
		first++
	}
	verb := finite(s, e)
	return listPlace{
		head:  IsMember(strings.TrimRight(tokens[len(tokens)-1], ".,;:!?")),
		inner: !verb,
		tail:  IsMember(strings.TrimRight(tokens[first], ".,;:!?")),
	}
}

// longestRun answers the most elements in a row that read as one list, and
// the first and the last of them. This is the earliest ends on a member.
// The last opens on one, and each between is a bare member.
func longestRun(places []listPlace) (best, first, last int) {
	for i := range places {
		if !places[i].head {
			continue
		}
		for j := i + 1; j < len(places); j++ {
			if places[j].tail && j-i+1 > best {
				best, first, last = j-i+1, i, j
			}
			if !places[j].inner || !places[j].tail {
				break
			}
		}
	}
	return best, first, last
}

// parenthetical matches a parenthesis with no parenthesis inside it.
var parenthetical = regexp.MustCompile(`\([^()]*\)`)

// ListSpan answers the part of one sentence that an enumeration takes out: a
// coordinated run of members, or a list behind a complete-set frame. A
// parenthesis is read as a list of its own. The span is the parenthesis, or
// the run behind its lead-in, when the sentence keeps a statement without
// it. Otherwise the span is the whole sentence: the list is its content.
func ListSpan(sentence string) (Span, bool) {
	whole := Span{0, len(sentence)}
	masked := ste.Masked(sentence)
	end := len(strings.TrimRight(sentence, " \t\n.!?"))
	if end <= 0 {
		return Span{}, false
	}
	outer, outerMasked := []byte(sentence), []byte(masked)
	var asides [][]int
	for _, loc := range parenthetical.FindAllStringIndex(masked, -1) {
		asides = append(asides, loc)
		for i := loc[0]; i < loc[1]; i++ {
			outer[i], outerMasked[i] = ' ', ' '
		}
	}
	s := syntax.Parse(masked, inlineCode.FindAllStringIndex(sentence, -1))
	keeps := func(sp Span) (Span, bool) {
		if keepsWords(sentence, []Span{sp}) {
			return sp, true
		}
		return whole, true
	}
	if a, b, ok := coordinated(string(outer), string(outerMasked), s, 0, end); ok {
		if sp, ok := behindLeadIn(masked, a, b, end); ok {
			return keeps(sp)
		}
		return whole, true
	}
	for _, loc := range asides {
		if _, _, ok := coordinated(sentence, masked, s, loc[0]+1, loc[1]-1); ok {
			return keeps(Span{loc[0], loc[1]})
		}
	}
	if framed(string(outer), string(outerMasked), end) {
		return whole, true
	}
	return Span{}, false
}

// coordinated answers the run of at least MinMembers elements in
// text[from:to], from its first member to its last.
func coordinated(text, masked string, s *syntax.Sentence, from, to int) (int, int, bool) {
	elements := elementsOf(text, masked, from, to, boundarySplit)
	places := make([]listPlace, len(elements))
	for i, e := range elements {
		places[i] = placeOf(text, s, e)
	}
	n, first, last := longestRun(places)
	if n < MinMembers {
		return 0, 0, false
	}
	head, tail := elements[first], elements[last]
	start, end := head.start, tail.end
	for _, loc := range tokenPattern.FindAllStringIndex(text[head.start:head.end], -1) {
		if IsMember(strings.TrimRight(text[head.start+loc[0]:head.start+loc[1]], ".,;:!?")) {
			start = head.start + loc[0]
		}
	}
	for _, loc := range tokenPattern.FindAllStringIndex(text[tail.start:tail.end], -1) {
		tok := text[tail.start+loc[0] : tail.start+loc[1]]
		if IsMember(strings.TrimRight(tok, ".,;:!?")) {
			end = tail.start + loc[0] + len(strings.TrimRight(tok, ".,;:!?"))
			break
		}
	}
	return start, end, true
}

// dashes open or close an aside.
var dashes = []string{"—", "–", "--"}

// behindLeadIn answers the span of a run that a lead-in word opens, such as
// "such as `a`, `b` and `c`". The span takes the comma or the dash before the
// lead-in, and the comma or the dash that closes the aside.
func behindLeadIn(masked string, a, b, end int) (Span, bool) {
	before := strings.TrimRight(masked[:a], " ")
	lower := strings.ToLower(before)
	open := -1
	for _, w := range listTable.WordsOf("lead-in") {
		phrase := strings.ReplaceAll(w, "_", " ")
		if strings.HasSuffix(lower, phrase) && (len(lower) == len(phrase) || !isLetter(lower[len(lower)-len(phrase)-1])) {
			open = len(before) - len(phrase)
			break
		}
	}
	if open < 0 {
		return Span{}, false
	}
	pre := strings.TrimRight(masked[:open], " ")
	closer := ""
	if strings.HasSuffix(pre, ",") {
		open, closer = len(pre)-1, ","
	}
	for _, d := range dashes {
		if strings.HasSuffix(pre, d) {
			open, closer = len(pre)-len(d), d
		}
	}
	if closer != "" && b < end {
		if i := strings.Index(masked[b:end], closer); i >= 0 {
			b += i + len(closer)
		}
	}
	return Span{open, b}, true
}

// framed reports a set noun, then a complete-set word or a colon, then a list
// of at least MinFramedMembers noun phrases.
func framed(sentence, masked string, end int) bool {
	s := syntax.Parse(masked, inlineCode.FindAllStringIndex(sentence, -1))
	for i, w := range s.Words {
		if w.Tag != "NNS" && w.Tag != "NNPS" {
			continue
		}
		at, ok := frameEnd(masked, w.End)
		if !ok || at >= end {
			continue
		}
		elements := elementsOf(sentence, masked, at, end, listSplit)
		if len(elements) < MinFramedMembers {
			continue
		}
		every := true
		for n, e := range elements {
			place := placeOf(sentence, s, e)
			if !nounPhrase(s, i+1, e) && !place.tail || finite(s, e) {
				every = false
			}
			// A joiner after "between" pairs the ends of one phrase. It divides no list.
			if n < len(elements)-1 && pairs.MatchString(sentence[e.start:e.end]) {
				every = false
			}
		}
		if every {
			return true
		}
	}
	return false
}

// pairs matches a word that takes both ends of a pair as one phrase.
var pairs = regexp.MustCompile(`(?i)\b(?:between|both)\b`)

// finite reports an element that holds a finite verb, which makes it a clause.
func finite(s *syntax.Sentence, e element) bool {
	for _, w := range s.Words {
		if w.Start >= e.start && w.Start < e.end && (w.Tag == "VBZ" || w.Tag == "VBP" || w.Tag == "VBD" || w.Tag == "MD") {
			return true
		}
	}
	return false
}

// frameEnd answers where the list after a set noun opens: past a word of the
// complete-set class, or past a colon.
func frameEnd(masked string, at int) (int, bool) {
	rest := masked[at:]
	trimmed := strings.TrimLeft(rest, " ")
	skip := len(rest) - len(trimmed)
	if strings.HasPrefix(trimmed, ":") {
		return at + skip + 1, true
	}
	lower := strings.ToLower(trimmed)
	for _, w := range listTable.WordsOf("complete-set") {
		phrase := strings.ReplaceAll(w, "_", " ")
		if strings.HasPrefix(lower, phrase) && (len(lower) == len(phrase) || !isLetter(lower[len(phrase)])) {
			return at + skip + len(phrase), true
		}
	}
	return 0, false
}

// nounPhrase reports an element that opens a noun phrase and holds a noun.
func nounPhrase(s *syntax.Sentence, from int, e element) bool {
	opened, noun := false, false
	for _, w := range s.Words[from:] {
		if w.Start < e.start {
			continue
		}
		if w.Start >= e.end {
			break
		}
		if !opened {
			opened = true
			switch {
			case w.Tag == "DT", w.Tag == "PRP$", w.Tag == "CD", w.Tag == "VBG",
				strings.HasPrefix(w.Tag, "JJ"), strings.HasPrefix(w.Tag, "NN"):
			default:
				return false
			}
		}
		if strings.HasPrefix(w.Tag, "NN") {
			noun = true
		}
	}
	return noun
}

// FrameKey answers a sentence with each member written as one mark. Sentences
// with one key differ only in the member they name, so they say nothing about
// it: a run of them is an enumeration. A sentence that says something
// different about each member is a description, and has a key of its own.
func FrameKey(sentence string) (string, bool) {
	if _, _, ok := firstMember(sentence); !ok {
		return "", false
	}
	tokens := tokenPattern.FindAllString(sentence, -1)
	words := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		bare := strings.Trim(tok, ".,;:!?")
		if IsMember(bare) || IsMember(tok) {
			words = append(words, "\x00")
			continue
		}
		words = append(words, strings.ToLower(bare))
	}
	return strings.Join(words, " "), true
}

// Span is a byte range of a text.
type Span struct{ Start, End int }

// SentenceSpans answers the byte span of each sentence of prose.
func SentenceSpans(prose string) []Span {
	var out []Span
	at := 0
	flat := strings.ReplaceAll(prose, "\n", " ")
	for _, s := range ste.Sentences(flat) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		i := strings.Index(flat[at:], s)
		if i < 0 {
			continue
		}
		out = append(out, Span{at + i, at + i + len(s)})
		at += i + len(s)
	}
	return out
}

// ProseCut is the part of prose an enumeration takes out, and the sentence it quotes.
type ProseCut struct {
	Span
	Detail string
}

// ProseCuts answers the part of each sentence that an enumeration takes out
// (see ListSpan), and each whole sentence of an enumeration split across
// sentences.
func ProseCuts(prose string) []ProseCut {
	spans := SentenceSpans(prose)
	cut := make([]*Span, len(spans))
	for i, sp := range spans {
		if in, ok := ListSpan(prose[sp.Start:sp.End]); ok {
			cut[i] = &Span{sp.Start + in.Start, sp.Start + in.End}
		}
	}
	keys := make([]string, len(spans))
	for i, sp := range spans {
		keys[i], _ = FrameKey(prose[sp.Start:sp.End])
	}
	for _, run := range SameKeyRuns(keys) {
		for i := run[0]; i < run[1]; i++ {
			whole := spans[i]
			cut[i] = &whole
		}
	}
	var out []ProseCut
	for i, sp := range spans {
		if cut[i] != nil {
			out = append(out, ProseCut{Span: *cut[i], Detail: strings.TrimSpace(prose[sp.Start:sp.End])})
		}
	}
	return out
}

// SameKeyRuns answers each run of at least MinMembers consecutive keys that are
// equal and not empty, as a half-open range of indexes.
func SameKeyRuns(keys []string) [][2]int {
	var out [][2]int
	for i := 0; i < len(keys); {
		j := i + 1
		for j < len(keys) && keys[i] != "" && keys[j] == keys[i] {
			j++
		}
		if keys[i] != "" && j-i >= MinMembers {
			out = append(out, [2]int{i, j})
		}
		i = j
	}
	return out
}

// CutProse answers prose with every span taken out, and the blanks around each cut closed.
func CutProse(prose string, spans []Span) string {
	sort.Slice(spans, func(a, b int) bool { return spans[a].Start < spans[b].Start })
	var b strings.Builder
	at := 0
	for _, sp := range spans {
		if sp.Start < at {
			continue
		}
		kept := prose[at:sp.Start]
		// A cut before a closing mark leaves no blank in front of it.
		if sp.End < len(prose) && closesClause(prose[sp.End]) {
			kept = strings.TrimRight(kept, " \t\n")
		}
		b.WriteString(kept)
		at = sp.End
	}
	b.WriteString(prose[at:])
	return strings.Join(strings.Fields(b.String()), " ")
}

// closesClause reports a mark that takes no blank before it.
func closesClause(c byte) bool { return strings.IndexByte(".,;:!?)", c) >= 0 }
