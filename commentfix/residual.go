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
		e := closeGap(src, at, stop)
		if e.Start < end {
			continue
		}
		e.Cut = []string{hit.Number}
		out = append(out, e)
		end = e.End
	}
	return out
}

// closeGap answers the edit that deletes the bytes from at up to stop,
// leaving the space a reader expects between words and none at all before
// punctuation or at the end of a line.
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
	case after == lineEnd, strings.ContainsRune(".,;:)!?", rune(src[after])):
		return edit.Edit{Start: before, End: after}
	case before < at || at == lineStart:
		return edit.Edit{Start: at, End: after}
	}
	return edit.Edit{Start: at, End: after, Text: " "}
}
