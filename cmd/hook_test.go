package cmd

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

// answer is the decoded response, plus the raw keys, because a caller reads
// this as JSON, where an absent key differs from an empty key.
type answer struct {
	raw  map[string]any
	out  map[string]any
	body string
}

func ask(t *testing.T, payload map[string]any, only ...string) answer {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	rules, ids, err := selectedRules(only)
	require.NoError(t, err)

	body := judge(data, rules, ids)
	if body == "" {
		return answer{}
	}
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &raw))
	out, _ := raw["hookSpecificOutput"].(map[string]any)
	return answer{raw: raw, out: out, body: body}
}

func write(path, content string) map[string]any {
	return map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input":      map[string]any{"file_path": path, "content": content},
	}
}

func TestAWriteIsRepairedAndLetThrough(t *testing.T) {
	got := ask(t, write("docs/a.md", "It has three plugins.\n"), "counts")

	require.NotNil(t, got.out)
	assert.Equal(t, "PreToolUse", got.out["hookEventName"])
	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated)
	assert.Equal(t, "It has plugins.\n", updated["content"])
	assert.Contains(t, got.out["additionalContext"], "three plugins")
	// A repair is not a refusal: the write goes through.
	assert.NotContains(t, got.body, "permissionDecision")
}

// Every key the payload carried survives, so a field this does not read is not
// dropped from the write.
func TestAKeyThisDoesNotReadSurvives(t *testing.T) {
	payload := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input": map[string]any{
			"file_path":   "docs/a.md",
			"old_string":  "before",
			"new_string":  "It has three plugins.",
			"replace_all": true,
		},
	}
	got := ask(t, payload, "counts")

	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated)
	assert.Equal(t, "It has plugins.", updated["new_string"])
	assert.Equal(t, "before", updated["old_string"])
	assert.Equal(t, true, updated["replace_all"])
}

func TestEveryEditOfAMultiEditIsRepaired(t *testing.T) {
	payload := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "MultiEdit",
		"tool_input": map[string]any{
			"file_path": "docs/a.md",
			"edits": []any{
				map[string]any{"old_string": "a", "new_string": "It has three plugins."},
				map[string]any{"old_string": "b", "new_string": "Nothing to repair."},
				map[string]any{"old_string": "c", "new_string": "There are four rules below."},
			},
		},
	}
	got := ask(t, payload, "counts")

	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated)
	edits, _ := updated["edits"].([]any)
	require.Len(t, edits, 3)
	assert.Equal(t, "It has plugins.", edits[0].(map[string]any)["new_string"])
	assert.Equal(t, "Nothing to repair.", edits[1].(map[string]any)["new_string"])
	assert.Equal(t, "There are rules below.", edits[2].(map[string]any)["new_string"])
}

// A finding no rewrite resolves refuses the write. A comment still over the cap
func TestAFindingNoRewriteResolvesRefusesTheWrite(t *testing.T) {
	src := "package p\n"
	for i := range 40 {
		src += fmt.Sprintf("// The loader reads step %d of the file and returns the record it names.\n", i)
	}
	src += "func x() {}\n"
	got := ask(t, write("a.go", src), "tombstones")

	require.NotNil(t, got.out)
	assert.Equal(t, "deny", got.out["permissionDecision"])
	assert.Contains(t, got.out["permissionDecisionReason"], "comment block of")
	assert.NotContains(t, got.body, "updatedInput")
}

// A tombstone alone on its own comment line is cut, and the write proceeds.
func TestAStrippableTombstoneIsCutRatherThanRefused(t *testing.T) {
	src := "// This used to read the flag.\nfunc x() {}\n"
	got := ask(t, write("a.go", src), "tombstones")

	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated)
	assert.Equal(t, "func x() {}\n", updated["content"])
	assert.NotContains(t, got.body, "permissionDecision")
	assert.Contains(t, got.out["additionalContext"], "rewrites")
}

// The rule selection reaches the payload, so a plugin naming a category is not
// handed another's verdict.
func TestOnlyTheNamedRuleJudgesTheWrite(t *testing.T) {
	src := "// This used to read the flag.\nfunc x() {}\n"
	assert.Empty(t, ask(t, write("a.go", src), "counts").body)
}

func TestNothingToRepairPrintsNothing(t *testing.T) {
	assert.Empty(t, ask(t, write("docs/a.md", "It has plugins.\n"), "counts").body)
}

// Every path that cannot be judged lets the write through untouched.
func TestAnUnreadablePayloadLetsTheWriteThrough(t *testing.T) {
	rules, ids, err := selectedRules([]string{"counts"})
	require.NoError(t, err)

	assert.Empty(t, judge([]byte("{"), rules, ids))
	assert.Empty(t, judge([]byte(`{"hook_event_name":"Stop"}`), rules, ids))
	assert.Empty(t, judge([]byte(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), rules, ids))
	assert.Empty(t, judge([]byte(`{"tool_name":"Write","tool_input":"not an object"}`), rules, ids))
	assert.Empty(t, ask(t, write("a.bin", "It has three plugins.\n"), "counts").body)
}

// The report is bounded, and says so rather than dropping the tail in silence.
func TestALongReportSaysItTrimmed(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	got := capped(lines)

	assert.Len(t, got, reportCap+1)
	assert.Equal(t, "... and more, not listed", got[len(got)-1])
	// Trimming must not scribble on the caller's slice.
	assert.Equal(t, "g", lines[reportCap])
}

func TestAnUnknownRuleIsRefusedBeforeAnyWriteIsJudged(t *testing.T) {
	_, _, err := selectedRules([]string{"nosuch"})
	require.Error(t, err)
}

var _ = slopfix.AllRules
