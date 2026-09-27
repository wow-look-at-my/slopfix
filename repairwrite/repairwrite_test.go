package repairwrite_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/repairwrite"
)

// fire drives the hook the way its launcher does.
func fire(t *testing.T, payload map[string]any) repairwrite.Result {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return repairwrite.Run(strings.NewReader(string(raw)))
}

// wrote puts a file on disk and answers a payload naming it.
func wrote(t *testing.T, name, content string) (string, map[string]any) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path, map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "Write",
		"tool_input":      map[string]any{"file_path": path},
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// The rule that reached disk unrepaired, which is why this hook exists.
func TestAWorkflowCommentBlockIsJoinedOnDisk(t *testing.T) {
	path, payload := wrote(t, ".github/workflows/ci.yml",
		"name: CI\n# one line about a thing\n# a second line about it\non: push\n")

	res := fire(t, payload)

	after := read(t, path)
	assert.NotContains(t, after, "# a second line about it\n", "the block is still two lines")
	assert.Contains(t, after, "one line about a thing a second line about it")
	assert.Equal(t, 0, res.Code, "a write that already happened is never refused")
}

// The model has to learn the bytes moved, or it quotes what it wrote.
func TestTheRepairIsNamedInTheContext(t *testing.T) {
	_, payload := wrote(t, ".github/workflows/ci.yml",
		"name: CI\n# one line about a thing\n# a second line about it\non: push\n")

	res := fire(t, payload)

	require.NotEmpty(t, res.Stdout)
	var out struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out))
	assert.Equal(t, "PostToolUse", out.HookSpecificOutput.HookEventName)
	assert.Contains(t, out.HookSpecificOutput.AdditionalContext, "Read it back")
}

// The control that proves the case above can fail.
func TestAFileWithNothingToRepairSaysNothing(t *testing.T) {
	path, payload := wrote(t, ".github/workflows/ci.yml", "name: CI\n# one line\non: push\n")
	before := read(t, path)

	res := fire(t, payload)

	assert.Empty(t, res.Stdout)
	assert.Equal(t, before, read(t, path), "an untouched file is not rewritten")
}

func TestAnotherEventIsLeftAlone(t *testing.T) {
	_, payload := wrote(t, ".github/workflows/ci.yml",
		"name: CI\n# one line about a thing\n# a second line about it\non: push\n")
	payload["hook_event_name"] = "PreToolUse"

	assert.Empty(t, fire(t, payload).Stdout)
}

func TestAToolThatWritesNoFileIsLeftAlone(t *testing.T) {
	_, payload := wrote(t, ".github/workflows/ci.yml",
		"name: CI\n# one line about a thing\n# a second line about it\non: push\n")
	payload["tool_name"] = "Bash"

	assert.Empty(t, fire(t, payload).Stdout)
}

// A repair that never ran has to say so: silence reads as a clean file.
func TestAMissingFileIsReportedOnStderr(t *testing.T) {
	res := fire(t, map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "Write",
		"tool_input":      map[string]any{"file_path": filepath.Join(t.TempDir(), "absent.md")},
	})

	assert.Contains(t, res.Stderr, "did not repair")
	assert.Equal(t, 0, res.Code)
}

func TestAnUnreadablePayloadIsLeftAlone(t *testing.T) {
	assert.Empty(t, repairwrite.Run(strings.NewReader("{ not json")).Stdout)
}
