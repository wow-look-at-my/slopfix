package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/forkscope"
)

const safetyLines = `///
/// # Safety
/// ` + "`handle`" + ` must be null or a pointer returned by
/// ` + "`converter_create`" + ` that has not already been freed.
`

const safetyDoc = "/// Free a converter handle\n" + safetyLines + freeFn

const freeFn = `#[no_mangle]
pub unsafe extern "C" fn converter_free(handle: *mut Handle) {
    if handle.is_null() {
        return;
    }
    let owned = Box::from_raw(handle);
    let streams = owned.streams.lock().unwrap();
    for (index, stream) in streams.iter() {
        stream.flush(*index);
        stream.close(*index);
    }
    drop(streams);
    owned.tokenizer.release();
    owned.parser.release();
    owned.decoder.release();
    owned.close();
    drop(owned);
}
`

// A Rust doc paragraph reads to its last line, so a sentence that wraps onto a
// line opening with inline code ends where its full stop is. Check and fix agree.
func TestARustDocSentenceReadsToItsLastLine(t *testing.T) {
	rules := []slopfix.Rule{slopfix.RuleComments}
	req := slopfix.Request{Path: "src/converter.rs", Content: safetyDoc, Rules: rules}
	assert.Zero(t, ofID(repairIDs(slopfix.Report(req)), commentfix.IDTail))

	repair := slopfix.Fix(req)
	assert.Zero(t, ofID(repairIDs(repair), commentfix.IDTail))
	require.Contains(t, repair.Text, "/// `converter_create` that has not already been freed.\n")
}

// In a fork that wrote the paragraph, check and fix agree the same way.
func TestARustDocSentenceTheForkWroteReadsToItsLastLine(t *testing.T) {
	base := "/// Free a converter handle\n" + freeFn
	owned := forkscope.Changed(base, safetyDoc)
	rules := []slopfix.Rule{slopfix.RuleComments}
	req := slopfix.Request{Path: "src/converter.rs", Content: safetyDoc, Rules: rules, Owned: owned}
	assert.Zero(t, ofID(repairIDs(slopfix.Report(req)), commentfix.IDTail))

	repair := slopfix.Fix(req)
	assert.Zero(t, ofID(repairIDs(repair), commentfix.IDTail))
	assert.Contains(t, repair.Text, "/// `converter_create` that has not already been freed.\n")
}
