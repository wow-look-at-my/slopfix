package slopfix

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/expect"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// pending answers each comment-length hit in the text as written. The repair
// fits such a block, so a report of the residue alone passed every essay.
func pending(req Request, already []tombstones.Hit) []tombstones.Hit {
	// A fixture is judged against its annotations, not against the repair.
	if expect.Parse(req.Content).Any() || tombstones.Borrowed(req.Path) {
		return nil
	}
	if kindOf(req.Path, req.Content) != fixer.Source || !wantsOf(req)(RuleComments) || !keepsOf(req)(commentfix.IDLength) {
		return nil
	}
	seen := set.New[int]()
	for _, hit := range already {
		if hit.ID == commentfix.IDLength {
			seen.Add(hit.LineNo)
		}
	}
	var out []tombstones.Hit
	for _, hit := range commentfix.CheckLength(req.Path, req.Content) {
		if !seen.Contains(hit.Line) {
			out = append(out, tombstones.Hit{ID: hit.ID, Tell: hit.Tell, Phrase: hit.Sentence, LineNo: hit.Line})
		}
	}
	return out
}
