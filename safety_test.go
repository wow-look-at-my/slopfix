package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// The doc comment the length repair deleted. Its heading carries the invariant
// for the sentence under it, and the repair cut the block to the heading alone.
func TestTheLengthRepairDoesNotDeleteADocBody(t *testing.T) {
	src := "/// # Safety\n" +
		"/// The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs.\n" +
		"fn f() {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "f.rs", Rules: []slopfix.Rule{slopfix.RuleComments}}).Text
	assert.Contains(t, out, "The caller must ensure", "the doc body was deleted:\n%s", out)
	assert.Contains(t, out, "no other thread writes through it", "the invariant clause was deleted:\n%s", out)
}

// The division restates the verb that governs the later clause, which the length repair relies on.
func TestTheSharedHeadDivisionShape(t *testing.T) {
	src := "The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs."
	assert.Equal(t, []string{"The caller must ensure that the pointer stays valid for reads of n bytes for the whole call.", "The caller must also ensure that no other thread writes through it while this function runs."}, ste.Sentences(ste.FixKeepingOpening(src)))
}

// A heading-led doc comment no repair can keep whole above a one-line body.
// The length repair leaves one whole opening sentence under the cap, and no
// finding is left after fix.
func TestTheLengthRepairLeavesAWholeOpeningSentenceAndNoFinding(t *testing.T) {
	src := "/// # Safety\n" +
		"/// The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs.\n" +
		"fn f() {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "f.rs"}).Text
	assert.Equal(t, out, slopfix.Fix(slopfix.Request{Content: out, Path: "f.rs"}).Text, "a second fix changed the text")
	assert.Empty(t, slopfix.CheckContent("f.rs", out), "a finding is left after fix:\n%s", out)

	var prose []string
	for _, line := range strings.Split(out, "\n") {
		if body, ok := strings.CutPrefix(line, "///"); ok && !strings.HasPrefix(strings.TrimSpace(body), "#") {
			prose = append(prose, strings.TrimSpace(body))
		}
	}
	opening := strings.TrimSpace(strings.Join(prose, " "))
	assert.Len(t, ste.Sentences(opening), 1, "the comment is not one sentence:\n%s", out)
	assert.True(t, strings.HasPrefix(opening, "The caller must ensure"), "the opening sentence went:\n%s", out)
	assert.True(t, strings.HasSuffix(opening, "."), "the opening sentence does not close:\n%s", out)
	assert.True(t, ste.StandsAlone(opening), "the opening sentence is not grammatical: %q", opening)
	for _, f := range ste.Check(opening, 1) {
		assert.NotEqual(t, ste.IDSentenceCap, f.ID, "%q: %s", opening, f.Detail)
	}
}

// A doc comment that opens on a heading is still repaired. The heading and
// the break line under it stay, and the repair leaves no comment finding.
func TestAHeadingLedDocCommentIsRepairedUnderItsHeading(t *testing.T) {
	src := "/// # Safety\n" +
		"///\n" +
		"/// The caller keeps the pointer valid for the whole call. No other thread writes through it. The buffer holds at least n bytes. The length counts bytes and not characters. A null pointer is never passed here.\n" +
		"unsafe fn first(p: *const u8, n: usize) -> u8 {\n" +
		"    let s = std::slice::from_raw_parts(p, n);\n" +
		"    let v = s[0];\n" +
		"    v\n" +
		"}\n"
	req := slopfix.Request{Content: src, Path: "f.rs", Rules: []slopfix.Rule{slopfix.RuleComments}}
	out := slopfix.Fix(req).Text
	assert.True(t, strings.HasPrefix(out, "/// # Safety\n///\n/// The caller keeps the pointer valid for the whole call."), "the heading or its break went:\n%s", out)
	assert.Empty(t, slopfix.CheckContent("f.rs", out), "a finding is left after fix:\n%s", out)
	assert.Contains(t, out, "unsafe fn first(p: *const u8, n: usize) -> u8 {\n", out)
	req.Content = out
	assert.Equal(t, out, slopfix.Fix(req).Text, "a second fix changed the text")
}

// A Rust doc block of sglang's tool_parser.rs. The bare `///` lines are
// paragraph breaks and `# Safety` is a heading, so neither joins the prose
// around it. One fix reaches a fixed point.
func TestARustDocHeadingAndItsBreaksStay(t *testing.T) {
	src := "/// Reset the parser state for reuse\n" +
		"///\n" +
		"/// # Safety\n" +
		"/// `handle` must be live; `text` must be NUL-terminated UTF-8; `result_json_out`\n" +
		"/// must be writable and its buffer freed with `sgl_free_string`.\n" +
		"#[no_mangle]\n" +
		"pub unsafe extern \"C\" fn sgl_tool_parser_reset(handle: *mut ToolParserHandle) {\n" +
		"    if handle.is_null() {\n" +
		"        return;\n" +
		"    }\n" +
		"    let handle_ref = &mut *handle;\n" +
		"    handle_ref.a.clear();\n" +
		"    handle_ref.b.clear();\n" +
		"    handle_ref.c.clear();\n" +
		"    handle_ref.d.clear();\n" +
		"}\n"
	once := slopfix.Fix(slopfix.Request{Content: src, Path: "src/tool_parser.rs", MaxCommentLines: 14}).Text
	twice := slopfix.Fix(slopfix.Request{Content: once, Path: "src/tool_parser.rs", MaxCommentLines: 14}).Text
	assert.Equal(t, once, twice, "a second fix changed the text")
	assert.Contains(t, once, "/// Reset the parser state for reuse\n///\n/// # Safety\n", once)
	assert.NotContains(t, once, "reuse # Safety", once)
}

// The sentence split that invented a fragment. The tail became a sentence of
// its own with a pronoun subject it never had: "It while this function runs."
func TestALongDocSentenceDoesNotSplitIntoAFragment(t *testing.T) {
	src := "/// # Safety\n" +
		"/// The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs.\n" +
		"fn f() {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "f.rs", Rules: []slopfix.Rule{slopfix.RuleSTE}}).Text
	assert.NotContains(t, out, "It while this function", "the split invented a sentence:\n%s", out)
	assert.Contains(t, out, "no other thread writes through it", "the clause was cut:\n%s", out)
	// Every part of the doc body must read as a sentence. "the pointer stays.
	var body string
	for _, line := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "/// The caller") {
			body = strings.TrimSpace(strings.TrimPrefix(t, "///"))
		}
	}
	if body != "" {
		sentences := ste.Sentences(body)
		assert.GreaterOrEqual(t, len(sentences), 2, "the over-cap sentence was left whole:\n%s", out)
		for _, s := range sentences {
			assert.True(t, ste.StandsAlone(strings.TrimSpace(s)), "a fragment was left: %q\n%s", s, out)
		}
	}
}

// A long doc-comment sentence the fixer used to leave standing. It has no main
// clause of its own, so the colon before its list is the boundary the division
// takes. The head closes, and the list it introduces stays whole.
func TestAColonEndsALongDocSentenceBeforeItsList(t *testing.T) {
	src := "/// Every flag a request may carry. An allowlist rather than a deny-list\n" +
		"/// because the flags that matter are the ones that turn a read into a write:\n" +
		"/// `-X POST`, `--method`, `--field`, `--input`.\n" +
		"const ALLOWED_FLAGS: &[&str] = &[\n" +
		"    \"--json\",\n" +
		"];\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "ci_host.rs", Rules: []slopfix.Rule{slopfix.RuleSTE}}).Text
	assert.Contains(t, out, "into a write.\n", "the sentence was not divided at its colon:\n%s", out)
	assert.Contains(t, out, "`-X POST`, `--method`, `--field`, `--input`", "the list was cut:\n%s", out)
}

// A long doc-comment sentence that opens on a participle and holds no main
// clause. The comma before its reason is the boundary the division takes, so
// neither half runs past the cap.
func TestACommaEndsALongFragmentDocSentence(t *testing.T) {
	src := "/// Compiled under `cfg(test)` off Linux as well, because the emitted argv IS the contract (the option order is what makes a later bind win, and what keeps the CI host-worker fd an option rather than a program argument) and an ordering only one host can assert is one that regresses quietly everywhere else.\n" +
		"fn f() {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "jail.rs", Rules: []slopfix.Rule{slopfix.RuleSTE}}).Text
	assert.Contains(t, out, "off Linux as well. Because the emitted argv IS the contract", "the sentence was not divided at its comma:\n%s", out)
}
