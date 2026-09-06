package slopfmt

import (
	"github.com/wow-look-at-my/slopfmt/counts"
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
// The wrap join runs last, on text whose counts are already gone, so a hit's
// byte span is never invalidated under it. A rewrite that changed a word is
// dropped: joining must only move newlines, and a result whose words differ is
// a bug in the splitter rather than a repair.
func Fix(content string) Repair {
	stripped, cut := counts.Strip(content)
	removed := make([]string, 0, len(cut))
	for _, hit := range cut {
		removed = append(removed, hit.Phrase)
	}

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
