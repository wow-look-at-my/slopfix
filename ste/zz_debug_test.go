package ste

import (
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// TestDebugRelativeClause reports what each division answers for the relative
// clause repairtruth case. It is a temporary probe, not a rule test.
func TestDebugRelativeClause(t *testing.T) {
	in := "A fenced block in which `MinMembers` or more lines that are not comments share a leading word and differ in the word after it, which is a usage block that walks through the subcommands of a program."
	masked := checkMask(in)
	whole := syntax.Parse(masked, nil)
	t.Logf("words=%d over=%d", WordCount(masked), overCap(in, SentenceWordCap))
	if out, ok := relativeInDivision(in, whole, SentenceWordCap); ok {
		t.Logf("relativeIn ok: %q", out)
	} else {
		t.Logf("relativeIn failed: %q", out)
	}
	if out, ok := relativeThisDivision(in, whole, SentenceWordCap); ok {
		t.Logf("relativeThis ok: %q", out)
	} else {
		t.Logf("relativeThis failed: %q", out)
	}
	t.Logf("capspec over=%d", overCap(in, SentenceWordCap))
	t.Logf("trace:\n%s", traceForce(in))
	t.Logf("fix=%q", Fix(in))
	t.Fail()
}
