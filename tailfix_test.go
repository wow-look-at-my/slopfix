package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
)

const safetyDoc = `/// Free a converter handle
///
/// # Safety
/// ` + "`handle`" + ` must be null or a pointer returned by
/// ` + "`converter_create`" + ` that has not already been freed.
#[no_mangle]
pub unsafe extern "C" fn converter_free(handle: *mut Handle) {}
`

// Every comments/tail finding check reports, fix repairs.
func TestFixRepairsEveryTailFindingCheckReports(t *testing.T) {
	rules := []slopfix.Rule{slopfix.RuleComments}
	req := slopfix.Request{Path: "src/converter.rs", Content: safetyDoc, Rules: rules}
	before := slopfix.Report(req)
	require.NotZero(t, ofID(repairIDs(before), commentfix.IDTail), "check reports the comment")

	repair := slopfix.Fix(req)
	assert.Zero(t, ofID(repairIDs(repair), commentfix.IDTail), "fix left a tail finding")
	after := slopfix.Report(slopfix.Request{Path: req.Path, Content: repair.Text, Rules: rules})
	assert.Zero(t, ofID(repairIDs(after), commentfix.IDTail), "check reports what fix wrote")
	assert.Contains(t, repair.Text, "pub unsafe extern \"C\" fn converter_free(handle: *mut Handle) {}")
}
