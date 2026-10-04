package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

const countsDoc = "Intro line.\nIt has three plugins.\nOutro line.\n"

func denial(t *testing.T, got answer) string {
	t.Helper()
	require.NotNil(t, got.out, "expected a refusal")
	require.Equal(t, "deny", got.out["permissionDecision"], got.body)
	reason, _ := got.out["permissionDecisionReason"].(string)
	return reason
}

// applied answers the file the tool writes from the payload the hook answered.
func applied(t *testing.T, before string, got answer) string {
	t.Helper()
	require.NotNil(t, got.out, "expected a rewrite")
	assert.NotContains(t, got.body, "permissionDecision")
	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated, got.body)
	edits, _ := updated["edits"].([]any)
	if edits == nil {
		edits = []any{updated}
	}
	out := before
	for _, e := range edits {
		m, _ := e.(map[string]any)
		old, _ := m["old_string"].(string)
		text, _ := m["new_string"].(string)
		require.Equal(t, 1, strings.Count(out, old), "the tool places %q a single time", old)
		out = strings.Replace(out, old, text, 1)
	}
	return out
}

// fixed answers what `slopfix fix` writes for src under the named rule categories.
func fixed(t *testing.T, path, src string, only ...string) string {
	t.Helper()
	rules, ids, err := selectedRules(only)
	require.NoError(t, err)
	return slopfix.Fix(slopfix.Request{Content: src, Path: path, Rules: rules, IDs: ids, MaxCommentLines: hookMaxLines}).Text
}

func TestAnEditToAnAutoFixableLineIsRewritten(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editOf(path, "It has three plugins.", "It has three good plugins."), "counts")

	hand := "Intro line.\nIt has three good plugins.\nOutro line.\n"
	want := fixed(t, path, hand, "counts")
	require.NotEqual(t, hand, want, "the edited line carries a finding fix repairs")
	assert.Equal(t, want, applied(t, countsDoc, got))
	assert.Contains(t, got.out["additionalContext"], path+":2: [")
	assert.Contains(t, got.out["additionalContext"], "counts")
}

// The case that was refused: a line holding a finding fix repairs, and an Edit
// that appends a sentence to it. The write goes through as fix writes it.
func TestAnEditAppendingToAFlaggedLineIsRewrittenNotRefused(t *testing.T) {
	line := "- Every report ends with the list of open PRs in merge order. Every PR on it is checked with `gh pr view --json state` in the same turn, right before the list goes out. A merged or closed PR is dropped from the list and from the whole report: no link, no \"has merged\" note. I linked gh-wait-ci PR 25 in a report to say it had merged, and the user read the link as a stale PR and stopped reading. I twice listed PRs the user had already merged, then overcorrected by writing a rule to stop listing PRs at all. The user: \"you ALWAYS give me a list of PRs for the merge order, YOU JUST NEED TO FUCKING CHECK THEM BEFORE GIVING THEM TO ME\".\n"
	doc := "# Miscellaneous Guidelines\n\n## General Development Rules\n" + line
	path := onDisk(t, "CLAUDE.misc.md", doc)
	require.NotEqual(t, doc, fixed(t, path, doc), "the edited line carries a finding fix repairs")

	old := "and the user read the link as a stale PR and stopped reading."
	text := old + " The user will now revert any merged PR I send."
	got := ask(t, editOf(path, old, text))

	hand := strings.Replace(doc, old, text, 1)
	assert.Equal(t, fixed(t, path, hand), applied(t, doc, got))
	assert.Contains(t, got.out["additionalContext"], path+":4: [")
}

// An edit that performs a repair by hand ends as the repair fix writes.
func TestAHandRepairEndsAsTheRepairFixWrites(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editOf(path, "It has three plugins.", "It now has three plugins."), "counts")

	hand := "Intro line.\nIt now has three plugins.\nOutro line.\n"
	assert.Equal(t, fixed(t, path, hand, "counts"), applied(t, countsDoc, got))
	assert.NotContains(t, applied(t, countsDoc, got), "three")
}

// A hand rewording that clears the finding is put back as fix writes the line.
func TestAHandRewordingEndsAsTheRepairFixWrites(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editOf(path, "It has three plugins.", "It has some plugins."), "counts")

	assert.Equal(t, fixed(t, path, countsDoc, "counts"), applied(t, countsDoc, got))
}

// A rewrite keeps every line the edit did not write as it stands on disk.
func TestARewriteLeavesLinesTheEditDidNotWrite(t *testing.T) {
	doc := "It has three plugins.\n\nIt has three tools.\n"
	path := onDisk(t, "a.md", doc)
	got := ask(t, editOf(path, "three tools.", "three good tools."), "counts")

	out := applied(t, doc, got)
	assert.True(t, strings.HasPrefix(out, "It has three plugins.\n\n"), out)
	assert.NotContains(t, strings.TrimPrefix(out, "It has three plugins.\n\n"), "three")
}

func TestKeepWrittenTakesOnlyTheWrittenLines(t *testing.T) {
	written := writtenLines("a\nb\nc\n", "a\nB\nc\n")
	assert.Equal(t, "a\nX\nc\n", keepWritten("a\nB\nc\n", "Y\nX\nc\n", written))

	// A join that rewrites a line the edit never wrote is left out.
	written = writtenLines("a\nb\n", "a\nB\n")
	assert.Equal(t, "a\nB\n", keepWritten("a\nB\n", "a B\n", written))
}

func TestLineSpanGrowsUntilItIsUnique(t *testing.T) {
	span, text := lineSpan("x\ny\nx\n", "x\ny\nx\nz\n")
	assert.Equal(t, 1, strings.Count("x\ny\nx\n", span))
	assert.Equal(t, "x\ny\nx\nz\n", strings.Replace("x\ny\nx\n", span, text, 1))

	span, text = lineSpan("a\nb\nc\n", "a\nB\nc\n")
	assert.Equal(t, "b\n", span)
	assert.Equal(t, "B\n", text)
}

func TestAnEditBesideAnAutoFixableLineIsAllowed(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editOf(path, "Outro line.", "Closing line."), "counts")

	assert.NotContains(t, got.body, "deny")
}

func TestAnInsertBesideAnAutoFixableLineIsAllowed(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editOf(path, "Outro line.\n", "Middle line.\nOutro line.\n"), "counts")

	assert.NotContains(t, got.body, "deny")
}

// A warning has no repair, so fix leaves its line, and a hand edit of it goes through.
func TestAnEditToAFindingFixDoesNotChangeIsAllowed(t *testing.T) {
	path := onDisk(t, "a.md", "The file is read by the tool.\n\nThe tool writes the log.\n")
	got := ask(t, editOf(path, "read by", "parsed by"), "ste")

	assert.NotContains(t, got.body, "deny")
}

func TestAWriteReplacingAFlaggedLineIsRepaired(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	hand := "Intro line.\nIt has three good plugins.\nOutro line.\n"
	got := ask(t, write(path, hand), "counts")

	require.NotNil(t, got.out)
	assert.NotContains(t, got.body, "permissionDecision")
	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated, got.body)
	assert.Equal(t, fixed(t, path, hand, "counts"), updated["content"])
}

func TestAWriteKeepingAFlaggedLineIsAllowed(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, write(path, "Intro line.\nIt has three plugins.\nClosing line.\n"), "counts")

	assert.NotContains(t, got.body, "deny")
}

func TestAMultiEditTouchingAnAutoFixableLineIsRewritten(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	payload := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "MultiEdit",
		"tool_input": map[string]any{
			"file_path": path,
			"edits": []any{
				map[string]any{"old_string": "Intro", "new_string": "Opening"},
				map[string]any{"old_string": "line.", "new_string": "row.", "replace_all": true},
				map[string]any{"old_string": "three", "new_string": "3"},
			},
		},
	}
	got := ask(t, payload, "counts")
	hand := "Opening row.\nIt has 3 plugins.\nOutro row.\n"
	want := fixed(t, path, hand, "counts")
	if want == hand {
		// fix leaves the hand version alone, so the line is put back as fix repairs the original.
		want = "Opening row.\n" + strings.SplitAfter(fixed(t, path, countsDoc, "counts"), "\n")[1] + "Outro row.\n"
	}
	assert.Equal(t, want, applied(t, countsDoc, got))
	updated, _ := got.out["updatedInput"].(map[string]any)
	assert.Len(t, updated["edits"], 1)
	assert.Contains(t, got.out["additionalContext"], path+":2: [")
	assert.NotContains(t, got.out["additionalContext"], path+":1: [")
}

func TestAnEditTheToolCannotPlaceIsLeftToTheTool(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editOf(path, "line.", "row."), "counts")

	assert.NotContains(t, got.body, "deny")
}

func TestAnErrorComputingTheFixIsDenied(t *testing.T) {
	dir := t.TempDir()
	got := ask(t, editOf(dir, "a", "b"), "counts")

	reason := denial(t, got)
	assert.Contains(t, reason, "slopfix cannot compute what `slopfix fix "+dir+"` changes")
	assert.Contains(t, reason, "is a directory")
}

func TestAnUnreadableEditIsDenied(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	hand, got := handFix("Edit", []byte(`{"replace_all":"yes"}`), path, nil, nil, noForkLines)

	assert.Nil(t, hand)
	require.NotEmpty(t, got)
	assert.Contains(t, got[0], "cannot read the edits to "+path)
}

func TestANewFileIsOnlyRepaired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.md")
	got := ask(t, write(path, "It has three plugins.\n"), "counts")

	require.NotNil(t, got.out)
	assert.NotContains(t, got.body, "deny")
	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated)
	assert.Equal(t, "It has plugins.\n", updated["content"])
}

// A line no finding names is named by the category whose repair changes it.
func TestALineIsNamedByTheCategoryThatChangesIt(t *testing.T) {
	names, err := ruleIDsOn(slopfix.Request{Content: countsDoc, Path: "a.md"}, []int{2})

	require.NoError(t, err)
	assert.NotEmpty(t, names[2])
}

func TestChangedLinesIgnoresAnInsertion(t *testing.T) {
	assert.Empty(t, changedLines("a\nb\n", "a\nx\nb\n"))
	assert.Equal(t, []int{2}, changedLines("a\nb\nc\n", "a\nB\nc\n"))
	assert.Equal(t, []int{1}, changedLines("a\nb\n", "b\n"))
}

func editOf(path, old, new string) map[string]any {
	return map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input":      map[string]any{"file_path": path, "old_string": old, "new_string": new},
	}
}
