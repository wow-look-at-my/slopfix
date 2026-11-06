// overlong_repair.go fits an over-long comment block to the budget of its code.

package commentfix

import (
	"slices"
	"strings"

	"github.com/wow-look-at-my/slopfix/memo"
	"github.com/wow-look-at-my/slopfix/ste"
)

// repair answers repairUncached once for each block.
func repair(b block) []string {
	key := blockKey{text: strings.Join(b.text, "\n"), codeLines: b.codeLines, codeChars: b.codeChars, exact: b.exact, header: b.header}
	return slices.Clone(repaired.Get(key, func() []string { return repairUncached(b) }))
}

// blockKey holds every field the repair reads, and not the block's place.
type blockKey struct {
	text                 string
	codeLines, codeChars int
	exact, header        bool
}

// repaired keeps repair's answer for the rest of the run.
var repaired = memo.New[blockKey, []string](4096)

// repairUncached fits a block to its budget, and cuts pragma free text only when the prose cut is not enough.
func repairUncached(b block) []string {
	out := repairProse(b)
	if fitsCode(out, b) {
		return out
	}
	// The pragma stays. Its free text is the prose left to cut.
	bare := withoutPragmaProse(b.text)
	if !sameText(bare, b.text) {
		if out := repairProse(block{start: b.start, end: b.end, codeLines: b.codeLines, codeChars: b.codeChars, text: bare, exact: b.exact, header: b.header}); fitsCode(out, b) {
			return out
		}
	}
	// No cut fits without a fragment. The comment stays, and check reports none of it.
	return out
}

// repairProse rewrites a block's prose and puts its directive lines back verbatim.
//
// Every repair path rebuilds the block out of prose() alone, which drops the
// directives: the rewrite then REPLACED them. A lost //go:embed leaves the
// variable it filled empty, and the tests reading it pass on nothing.
func repairProse(b block) []string {
	lead, body, trail := splitDirectives(b.text)
	if out, ok := repairBlockComment(b); ok {
		return out
	}
	if len(lead) == 0 && len(trail) == 0 {
		return trim(b)
	}
	for len(body) > 0 && isBlankComment(body[len(body)-1]) {
		body = body[:len(body)-1]
	}
	if len(body) == 0 {
		return b.text
	}
	bodyBlock := block{start: b.start, end: b.end, codeLines: b.codeLines, codeChars: b.codeChars, text: body, exact: b.exact}
	// A /* */ body is cut as a block, so its closer survives the cut.
	if out, ok := repairBlockComment(bodyBlock); ok {
		return append(append(append([]string{}, lead...), out...), trail...)
	}
	kept := trim(bodyBlock)
	if len(trail) > 0 {
		kept = withSeparator(kept, trail)
		// The doc does not fit beside the directive, so its opening sentence stays with it.
		if !fitsCode(kept, b) {
			if opening, ok := fittedOpening(body, b); ok {
				kept = withSeparator(opening, trail)
			}
		}
	}
	out := make([]string, 0, len(lead)+len(kept)+len(trail))
	out = append(out, lead...)
	out = append(out, kept...)
	return append(out, trail...)
}

// withSeparator adds the bare marker line gofmt puts between a doc and the
// directive after it. The repair is measured as gofmt will leave it.
func withSeparator(kept, trail []string) []string {
	indent := trail[0][:len(trail[0])-len(strings.TrimLeft(trail[0], " \t"))]
	return append(append([]string{}, kept...), indent+"//")
}

// cutProse cuts the block's trailing prose until it fits, keeping the opening.
// A paragraph goes before a line does, and the opening paragraph always survives.
func cutProse(b block) []string {
	kept := withoutBanners(b.text)
	if _, over := judge(block{text: kept, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
		return kept
	}

	// Tighten before cutting. A padded comment fits after its filler is gone and
	// it is reflowed, and keeping the whole thought beats losing the last of it.
	if tightened, _, did := tighten(kept); did {
		if _, over := judge(block{text: tightened, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return tightened
		}
		kept = tightened
	}

	// The words can fit where the wrap does not. Laying them out at the budget's
	// own width drops none.
	if wider, did := widen(kept, max(floorChars, b.codeChars)); did {
		if _, over := judge(block{text: wider, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return wider
		}
	}

	fit := func(head []string, sentence, indent, marker string) ([]string, bool) {
		out := append(append([]string{}, head...), reflow(sentence, indent, marker, wrapWidth)...)
		return out, fitsCode(out, b)
	}
	// A later "that" clause states a condition of its own, so a cut that drops it keeps the verb that governs it.
	restating := func(head []string, sentence, indent, marker string) ([]string, bool) {
		return fitReflow(head, sentence, indent, marker, b)
	}
	if restated, ok := restatedOpening(kept, restating); ok {
		return restated
	}
	if out, ok := cutToFit(kept, b); ok {
		return out
	}
	uncut := kept
	for {
		next, ok := cutLastThought(kept)
		if !ok {
			break
		}
		kept = next
	}
	// The STE opening sentence reads best, then a clause cut.
	opening, whole := steOpening(kept, fit, false)
	var fits [][]string
	if whole && fitsCode(opening, b) {
		fits = append(fits, opening)
	}
	if clause, ok := clauseFit(b); ok {
		fits = append(fits, clause)
	}
	if out, ok := preferred(fits); ok {
		return out
	}
	// No cut fits, so the opening sentence divides until its first part does.
	if shorter, ok := steOpening(kept, fit, true); ok && fitsCode(shorter, b) {
		return shorter
	}
	// Every cut is refused. The whole prose goes on one line: the line count is
	// what the code beneath it weighs, and no word is dropped to reach one.
	if wider, did := widen(kept, oneLine); did {
		if _, over := judge(block{text: wider, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return wider
		}
	}
	// No cut lands on a sentence end, so the prose divides into sentences and the cut runs again.
	if divided, ok := divideProse(uncut); ok {
		if out, ok := cutToFit(divided, b); ok {
			return out
		}
	}
	// The floor admits a sentence under the cap on one line, so the opening sentence takes one.
	wide := func(head []string, sentence, indent, marker string) ([]string, bool) {
		out := append(append([]string{}, head...), reflow(sentence, indent, marker, oneLine)...)
		return out, fitsCode(out, b)
	}
	if opening, ok := steOpening(kept, wide, true); ok && fitsCode(opening, b) {
		return opening
	}
	return kept
}

// sameText compares runs of lines by what they say. A line count cannot: a
// repair often keeps the count and still shortens the text.
func sameText(a, b []string) bool {
	return strings.Join(a, "\n") == strings.Join(b, "\n")
}

// clauseFit cuts the block back to the last clause boundary that fits, and
// closes what it keeps with a period. A whole clause still reads as a sentence.
func clauseFit(b block) ([]string, bool) {
	marker, indent, ok := commentShape(b.text)
	if !ok {
		return nil, false
	}
	// The cut stays inside the first paragraph. Joined across a paragraph break or a list, the prose only looks shorter.
	var body []string
	// A heading and its break lines stay, and the cut runs in the prose under them.
	head := sectionHead(prose(b.text))
	for _, para := range paragraphs(prose(b.text)) {
		if para.blank || para.verbatim {
			continue
		}
		if para.list() {
			return nil, false
		}
		body = para.lines
		break
	}
	// A line that ends a thought with no stop gets one, so a cut keeps whole sentences.
	text := unwrapAside(strings.Join(strings.Fields(closeThoughts(body)), " "))
	if head != nil {
		if out, ok := clauseUnder(head, text, indent, marker, b); ok {
			return out, true
		}
	}
	// Where no cut fits under the heading, the cut goes alone.
	return clauseUnder(nil, text, indent, marker, b)
}

// clauseUnder cuts text back to the last clause boundary that fits under the
// head lines.
func clauseUnder(head []string, text, indent, marker string, b block) ([]string, bool) {
	for _, cut := range clauseCuts(text) {
		kept := strings.TrimRight(text[:cut], " ,;:-—–")
		words := strings.Fields(kept)
		if len(words) == 0 || !balanced(kept) || dangling.Contains(strings.ToLower(words[len(words)-1])) {
			continue
		}
		// A cut at a colon keeps the statement the colon elaborates.
		if !closesWhole(kept) && !(cut < len(text) && text[cut] == ':') {
			continue
		}
		if !endsSentence(kept) {
			kept += "."
		}
		if out, ok := fitReflow(head, kept, indent, marker, b); ok {
			return out, true
		}
	}
	return nil, false
}

// unwrapAside drops the parentheses around a comment that is one aside, so a
// cut inside it leaves the pair balanced.
func unwrapAside(text string) string {
	inner, ok := strings.CutPrefix(text, "(")
	if !ok {
		return text
	}
	end := strings.TrimSuffix(inner, ".")
	inner, ok = strings.CutSuffix(end, ")")
	if !ok || strings.ContainsAny(inner, "()") {
		return text
	}
	return strings.TrimSuffix(inner, ".") + "."
}

// closesWhole reports whether the last sentence of text holds a main clause, so
// a cut there leaves a sentence: "If the cache is cold" does not.
func closesWhole(text string) bool {
	sentences := ste.Sentences(text)
	return len(sentences) > 0 && standsAlone(sentences[len(sentences)-1])
}
