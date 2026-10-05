package cmd

import (
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// reportFinding is the JSON wire contract, held apart from ste.Finding so a rename cannot change it.
type reportFinding struct {
	// ID names the rule, the way a compiler names a warning.
	ID string `json:"id"`
	// Line is where the finding starts. EndLine repeats it on a finding that covers a single line.
	Line    int `json:"line"`
	EndLine int `json:"endLine"`
	// Rule says what the ID stands for. Detail quotes the text, and Fix names the repair.
	Rule   string `json:"rule"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
	// Repairable reports whether slopfix repairs this defect.
	Repairable bool `json:"repairable"`
	// Severity is "error", which fails a check, or "warning", which does not.
	Severity string `json:"severity"`
}

// reportOutput is what `check --json` writes for text on stdin.
type reportOutput struct {
	// Path echoes what was judged, so a caller batching calls tells them apart.
	Path string `json:"path"`
	// Text is the repaired text. Only a repair writes it.
	Text *string `json:"text,omitempty"`
	// Removed names each span the repair cut out.
	Removed  []string        `json:"removed,omitempty"`
	Findings []reportFinding `json:"findings"`
}

// wireFindings flattens every finding a repair carries. A nil slice marshals
// to null, which crashes a caller that reads its length.
func wireFindings(findings []ste.Finding, kept []tombstones.Hit) []reportFinding {
	out := []reportFinding{}
	for _, finding := range findings {
		out = append(out, wireFinding(finding))
	}
	for _, hit := range kept {
		out = append(out, wireFinding(ste.Finding{Line: hit.LineNo, ID: hit.ID, Rule: hit.Tell, Detail: hit.Phrase, Fix: hit.Fix}))
	}
	return out
}

func wireFinding(finding ste.Finding) reportFinding {
	end := finding.EndLine
	if end < finding.Line {
		end = finding.Line
	}
	return reportFinding{
		ID:         finding.ID,
		Line:       finding.Line,
		EndLine:    end,
		Rule:       finding.Rule,
		Detail:     finding.Detail,
		Fix:        finding.Fix,
		Repairable: slopfix.Repairable(finding.ID) && !ste.ByHand(finding.Fix),
		Severity:   severity(finding),
	}
}

func severity(finding ste.Finding) string {
	if finding.Warning() {
		return ste.SeverityWarning
	}
	return "error"
}
