package slopfmt

import (
<<<<<<< HEAD
	"os"

	"github.com/wow-look-at-my/slopfmt/counts"
	"github.com/wow-look-at-my/slopfmt/markdown"
=======
	"github.com/wow-look-at-my/slopfmt/counts"
>>>>>>> origin/master
	"github.com/wow-look-at-my/slopfmt/ste"
)

// Repair is what a caller gets back for a piece of text: the text as this
// binary would write it, and what no rewrite can repair.
//
// A hook reads Text to replace the write it was about to allow, and Findings to
// refuse one. Nothing else needs a rule of its own.
type Repair struct {
	// Text is the repaired document. It equals the input when Changed is false.
	Text string `json:"text"`
	// Changed reports whether any rewrite applied.
	Changed bool `json:"changed"`
	// Removed names each count whose cardinal was cut.
	Removed []string `json:"removed,omitempty"`
	// Findings are what a reader must repair by hand.
	Findings []ste.Finding `json:"findings"`
}

// Fix repairs what a rewrite can repair and reports the rest.
//
<<<<<<< HEAD
// Three repairs run, in this order. The count strip goes first, on the source,
// so a hit's byte span is still valid. The wrap join follows. The prose repair
// runs inside the join, on each block's joined text, because a rule reads a
// paragraph as one sentence stream and a hand wrap hides half of it.
//
// The join must only move newlines. Format proves that on this document first,
// and a document it cannot prove is returned with its counts cut and nothing
// else. The prose repair is different in kind: it changes words on purpose,
// each one to the replacement Check names.
=======
// The wrap join runs last, on text whose counts are already gone, so a hit's
// byte span is never invalidated under it. A rewrite that changed a word is
// dropped: joining must only move newlines, and a result whose words differ is
// a bug in the splitter rather than a repair.
>>>>>>> origin/master
func Fix(content string) Repair {
	stripped, cut := counts.Strip(content)
	removed := make([]string, 0, len(cut))
	for _, hit := range cut {
		removed = append(removed, hit.Phrase)
	}

<<<<<<< HEAD
	repaired := stripped
	if _, safe := Format(stripped); safe {
		repaired = markdown.FormatFunc(stripped, ste.Fix)
	}
	return Repair{
		Text:     repaired,
		Changed:  repaired != content,
		Removed:  removed,
		Findings: Check(repaired),
	}
}

// FixFile repairs a file in place and reports what it did. It writes nothing
// when the repair leaves the document as it was.
func FixFile(path string) (Repair, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Repair{}, err
	}
	repair := Fix(string(content))
	if !repair.Changed {
		return repair, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return repair, err
	}
	if err := os.WriteFile(path, []byte(repair.Text), info.Mode().Perm()); err != nil {
		return repair, err
	}
	return repair, nil
}
=======
	formatted, safe := Format(stripped)
	if !safe {
		formatted = stripped
	}
	return Repair{
		Text:     formatted,
		Changed:  formatted != content,
		Removed:  removed,
		Findings: Check(formatted),
	}
}
>>>>>>> origin/master
