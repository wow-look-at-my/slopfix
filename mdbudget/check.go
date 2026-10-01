package mdbudget

import (
	"fmt"
	"unicode/utf8"

	"github.com/wow-look-at-my/slopfix/ste"
)

// ID names the rule `slopfix check` reports for an instruction file past its budget.
const ID = "budget/instruction-file"

// Check reports an instruction file that is over its budget, or one with
// almost no room left. The path decides whether the file is one at all.
func Check(path, content string) []ste.Finding {
	limit := budget()
	if limit == 0 || !isInstructionFile(path) {
		return nil
	}
	chars := utf8.RuneCountInString(content)
	finding := ste.Finding{
		Line: 1,
		ID:   ID,
		Fix:  "Move whole sections into docs/ word for word, and leave a line that points at each.",
	}
	switch {
	case chars > limit:
		finding.Rule = fmt.Sprintf("%d characters, over the %d budget every request re-sends", chars, limit)
	case chars >= nearLimit(limit):
		finding.Rule = fmt.Sprintf("%d characters, with almost no room left under the %d budget", chars, limit)
		finding.Severity = ste.SeverityWarning
	default:
		return nil
	}
	return []ste.Finding{finding}
}
