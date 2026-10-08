// residual.go closes the repair: whatever the table and the sentence cut left
// behind is deleted where it sits.
//
// The table rewrites what a swap can say in words. The sentence cut takes the
// rest. Neither reaches a number the extractor finds on a line no paragraph
// covers, such as an indented example inside a block comment. This pass reads
// the finding's own position and deletes those bytes. The result is a comment
// the rule reports nothing about.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
)

// residualPasses guards the loop: a pass deletes bytes, so the text shrinks.
const residualPasses = 8

// clearResidual deletes every number Check still finds, and reports what it
// took. Each deletion is an edit inside the comment node the hit sits in.
func clearResidual(f *fixer.File) {
	for range residualPasses {
		hits := Check(f.Path, f.Text())
		if len(hits) == 0 {
			return
		}
		if res := f.ApplyComments(deletions(f.Text(), hits)); len(res.Applied) == 0 {
			// Nothing was deletable, so another pass finds the same text.
			return
		}
	}
}

// deletions answers an edit per hit that removes the number and closes the
// gap it leaves. An edit that would overlap the one before it waits for the
// next pass.
func deletions(src string, hits []Hit) []edit.Edit {
	var out []edit.Edit
	end := -1
	for _, hit := range hits {
		at := hit.Offset
		stop := at + len(hit.Number)
		if at < 0 || stop > len(src) {
			continue
		}
		at, stop = literalAround(src, at, stop)
		e := closeGap(src, at, stop)
		if e.Start < end {
			continue
		}
		e.Cut = []string{src[at:stop]}
		out = append(out, e)
		end = e.End
	}
	return out
}

// literalAround widens the digits from at up to stop to the whole numeric
// literal they belong to: the fraction after a decimal point. This also covers
// the groups after a thousands separator, an exponent, and a sign or power
// mark in front. Deleting only the digits a finding names leaves the rest
// standing as fragments such as ".874".
func literalAround(src string, at, stop int) (int, int) {
	for at > 0 {
		switch {
		case isDigit(src[at-1]):
			at--
		case at > 1 && strings.IndexByte(".,_", src[at-1]) >= 0 && isDigit(src[at-2]):
			at--
		case strings.IndexByte(".^", src[at-1]) >= 0:
			at--
		case strings.IndexByte("+-", src[at-1]) >= 0 && (at == 1 || isSpace(src[at-2])):
			at--
		default:
			return at, extendLiteral(src, stop)
		}
	}
	return at, extendLiteral(src, stop)
}

// extendLiteral moves stop past the rest of the numeric literal it is inside.
func extendLiteral(src string, stop int) int {
	for stop < len(src) {
		switch {
		case isDigit(src[stop]):
			stop++
		case strings.IndexByte(".,_", src[stop]) >= 0 && stop+1 < len(src) && isDigit(src[stop+1]):
			stop += 2
		case (src[stop] == 'e' || src[stop] == 'E') && exponentAt(src, stop+1):
			stop++
			if src[stop] == '+' || src[stop] == '-' {
				stop++
			}
		default:
			return stop
		}
	}
	return stop
}

// exponentAt reports whether an exponent's optional sign and digits start at i.
func exponentAt(src string, i int) bool {
	if i < len(src) && (src[i] == '+' || src[i] == '-') {
		i++
	}
	return i < len(src) && isDigit(src[i])
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isSpace(b byte) bool { return b == ' ' || b == '\t' }

// closeGap answers the edit that deletes the bytes from at up to stop.
// That closeGap is leaving the space a reader expects between words and
// none at all before punctuation or at the end of a line.
func closeGap(src string, at, stop int) edit.Edit {
	lineStart := strings.LastIndexByte(src[:at], '\n') + 1
	lineEnd := len(src)
	if i := strings.IndexByte(src[stop:], '\n'); i >= 0 {
		lineEnd = stop + i
	}
	before := at
	for before > lineStart && (src[before-1] == ' ' || src[before-1] == '\t') {
		before--
	}
	after := stop
	for after < lineEnd && (src[after] == ' ' || src[after] == '\t') {
		after++
	}
	switch {
	case after < lineEnd && lineStart > 0 && strings.Trim(src[lineStart:before], " \t*") == "" && strings.ContainsRune(".,;:)!?", rune(src[after])):
		// The number opened a continuation line, so the mark after it joins the line above rather than standing on a line.
		return edit.Edit{Start: len(strings.TrimRight(src[:lineStart-1], " \t")), End: after}
	case after == lineEnd, strings.ContainsRune(".,;:)!?", rune(src[after])):
		return edit.Edit{Start: before, End: after}
	case before < at || at == lineStart:
		return edit.Edit{Start: at, End: after}
	}
	return edit.Edit{Start: at, End: after, Text: " "}
}
