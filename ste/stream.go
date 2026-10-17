// stream.go runs the sentence-cap repair over a text in one pass.
//
// A division rewrites one sentence. The text before it does not change
// again, and a sentence that no division reads stays so. The pass therefore
// goes on from the sentence it rewrote and does not read the text again.
//
// A long sentence divides window by window. Each division lands within the
// first cap words, so the words past the window cannot move it. The cost of
// a sentence is then in proportion to its length.
package ste

import (
	"slices"
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// windowCaps is the size of a window, in caps.
const windowCaps = 8

// fixSentenceCap divides every over-cap sentence. It tries the clause
// boundaries first and then the word boundaries. A sentence that no division
// reads is cut between words. That leaves no finding standing.
func fixSentenceCap(prose string, d capSpec) string {
	budget := len(strings.Fields(prose)) + 1
	var b strings.Builder
	masked := mask(prose)
	divideAll(&b, prose, masked, opaque(prose, masked), d.cap, &budget)
	return forceSentenceCap(b.String(), d)
}

// divideAll writes text with each over-cap sentence divided at a clause
// boundary, and every blank between sentences as it stands. masked and off
// are mask and opaque of text.
func divideAll(out *strings.Builder, text, masked string, off [][]int, limit int, budget *int) {
	spans := sentenceSpans(text)
	near := spansNear(off)
	last := 0
	for _, span := range spans {
		start, end := span[0], span[1]
		out.WriteString(text[last:start])
		out.WriteString(divideOne(text[start:end], masked[start:end], near(start, end), limit, budget))
		last = end
	}
	out.WriteString(text[last:])
}

// divideOne divides one sentence at a clause boundary, then the sentences the
// division wrote, until none is over the cap or no boundary divides it.
func divideOne(source, masked string, off [][]int, limit int, budget *int) string {
	if *budget <= 0 || WordCount(masked) <= limit {
		return source
	}
	if cut := newCutter(source); cut.longerThan(windowCaps * limit) {
		return divideLong(source, cut, limit, budget)
	}
	rewritten, ok := divideAt(source, masked, off)
	if !ok {
		return source
	}
	*budget--
	var b strings.Builder
	m := mask(rewritten)
	divideAll(&b, rewritten, m, opaque(rewritten, m), limit, budget)
	return b.String()
}

// divideAt answers source divided at its best clause boundary.
func divideAt(source, masked string, off [][]int) (string, bool) {
	s := syntax.Parse(masked, off)
	if rewritten, ok := bestDivision(s, source); ok && keepsCodeSpans(source, rewritten) {
		return rewritten, true
	}
	if rewritten, ok := frontedDivision(s, source); ok && keepsCodeSpans(source, rewritten) {
		return rewritten, true
	}
	return "", false
}

// divideLong divides a long sentence window by window. A window that holds no
// clause boundary grows, up to a bound. Past the bound the rest of the
// sentence is left to the forced division.
func divideLong(source string, cut *cutter, limit int, budget *int) string {
	var out strings.Builder
	head, rest := "", source
	words := windowCaps * limit
	for *budget > 0 {
		at, ok := cut.after(len(source)-len(rest), max(words-len(strings.Fields(head)), limit))
		if !ok {
			break
		}
		window := head + rest[:at]
		m := mask(window)
		rewritten, ok := divideAt(window, m, opaque(window, m))
		if !ok {
			if words >= 4*windowCaps*limit {
				break
			}
			words *= 2
			continue
		}
		spans := sentenceSpans(rewritten)
		tail := spans[len(spans)-1][0]
		if tail == 0 {
			break
		}
		words = windowCaps * limit
		*budget--
		m = mask(rewritten[:tail])
		divideAll(&out, rewritten[:tail], m, opaque(rewritten[:tail], m), limit, budget)
		head, rest = rewritten[tail:], rest[at:]
	}
	// The rest is one sentence. A short rest divides as any sentence does. A
	// long one held no boundary in its windows, and the forced division takes it.
	rem := head + rest
	if newCutter(rem).longerThan(windowCaps * limit) {
		out.WriteString(rem)
		return out.String()
	}
	m := mask(rem)
	out.WriteString(divideOne(rem, m, opaque(rem, m), limit, budget))
	return out.String()
}

// forceSentenceCap divides each sentence still over the cap at a word boundary.
func forceSentenceCap(prose string, d capSpec) string {
	budget := len(strings.Fields(prose)) + 1
	var b strings.Builder
	forceAll(&b, prose, d, &budget)
	return b.String()
}

// forceAll writes text with each over-cap sentence divided between words, and
// every blank between sentences as it stands.
func forceAll(out *strings.Builder, text string, d capSpec, budget *int) {
	masked := checkMask(text)
	last := 0
	for _, span := range forceSpans(text, masked) {
		start, end := max(span[0], last), span[1]
		out.WriteString(text[last:start])
		out.WriteString(forceOne(text[start:end], masked[start:end], d, budget))
		last = end
	}
	out.WriteString(text[last:])
}

// forceSpans answers each sentence as Check reads it. A filled span ends in a
// blank where the prose has its mark, so the sentence takes the mark back.
func forceSpans(prose, masked string) [][2]int {
	spans := sentenceSpans(masked)
	for i, span := range spans {
		start, end := span[0], span[1]
		for start > 0 && masked[start-1] == ' ' && prose[start-1] != ' ' {
			start--
		}
		for end < len(prose) && masked[end] == ' ' && prose[end] != ' ' {
			end++
		}
		spans[i] = [2]int{start, end}
	}
	return spans
}

// forceOne divides one sentence between words, then the sentences the
// division wrote, until none is over the cap or no division reads it.
func forceOne(source, masked string, d capSpec, budget *int) string {
	if *budget <= 0 || WordCount(masked) <= d.cap {
		return source
	}
	if cut := newCutter(source); cut.longerThan(windowCaps * d.cap) {
		return forceLong(source, cut, d, budget)
	}
	return forceWhole(source, masked, d, budget)
}

// forceWhole is forceOne with no window.
func forceWhole(source, masked string, d capSpec, budget *int) string {
	if *budget <= 0 || WordCount(masked) <= d.cap {
		return source
	}
	rewritten, ok := forceDivision(source, masked, d)
	if !ok || overCap(rewritten, d.cap) >= overCap(source, d.cap) {
		return source
	}
	*budget--
	var b strings.Builder
	forceAll(&b, rewritten, d, budget)
	return b.String()
}

// probeForce is a temporary debugging hook.
var probeForce = func(string, int) {}

// forceLong divides a long sentence window by window. A window that no
// division reads hands the whole rest to forceWhole once.
func forceLong(source string, cut *cutter, d capSpec, budget *int) string {
	var out strings.Builder
	head, rest := "", source
	words := windowCaps * d.cap
	for *budget > 0 {
		at, ok := cut.after(len(source)-len(rest), max(words-len(strings.Fields(head)), d.cap))
		if !ok {
			probeForce("nocut", len(rest))
			break
		}
		window := head + rest[:at]
		masked := checkMask(window)
		rewritten, ok := forceDivision(window, masked, d)
		if !ok || overCap(rewritten, d.cap) >= overCap(window, d.cap) {
			probeForce("refused", words)
			// The end of a window can refuse a division the whole sentence takes. A wider window reads past it.
			if words >= 4*windowCaps*d.cap {
				break
			}
			words *= 2
			continue
		}
		spans := forceSpans(rewritten, checkMask(rewritten))
		tail := spans[len(spans)-1][0]
		if tail == 0 {
			probeForce("tail0", len(rewritten))
			break
		}
		probeForce("step", len(window))
		words = windowCaps * d.cap
		*budget--
		forceAll(&out, rewritten[:tail], d, budget)
		head, rest = rewritten[tail:], rest[at:]
	}
	// The rest is one sentence: short, or held no division in its window.
	rem := head + rest
	out.WriteString(forceWhole(rem, checkMask(rem), d, budget))
	return out.String()
}

// spansNear answers, for each sentence in order, the spans of off that touch
// it, moved so that the sentence starts at zero. The sentences must come in
// order, so each span is read a bounded number of times.
func spansNear(off [][]int) func(start, end int) [][]int {
	sorted := slices.Clone(off)
	slices.SortFunc(sorted, func(a, b []int) int { return a[0] - b[0] })
	next := 0
	var open [][]int
	return func(start, end int) [][]int {
		for next < len(sorted) && sorted[next][0] < end {
			open = append(open, sorted[next])
			next++
		}
		kept := open[:0]
		var out [][]int
		for _, span := range open {
			if span[1] <= start {
				continue
			}
			kept = append(kept, span)
			if span[0] < end {
				out = append(out, []int{span[0] - start, span[1] - start})
			}
		}
		open = kept
		return out
	}
}
