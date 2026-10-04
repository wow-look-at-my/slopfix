package bashclean

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postPayload(t *testing.T, command, cwd string, response map[string]any) *strings.Reader {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"cwd":             cwd,
		"tool_input":      map[string]any{"command": command},
		"tool_response":   response,
	})
	require.NoError(t, err)
	return strings.NewReader(string(data))
}

func postOutput(t *testing.T, res HookResult) (map[string]any, string) {
	t.Helper()
	var out struct {
		Spec struct {
			Updated map[string]any `json:"updatedToolOutput"`
			Context string         `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out))
	return out.Spec.Updated, out.Spec.Context
}

func TestRunPostShowsAMappedReadAsReadShowsIt(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("one\ntwo\r\nthree\nfour\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "g.txt"), []byte("a\nb"), 0o644))
	ok := map[string]any{"stdout": "two\nthree\n", "stderr": "", "interrupted": false}
	cases := []struct{ cmd, stdout string }{
		{"sed -n 2,3p f.txt", "2\ttwo\n3\tthree"},
		{"head -n 1 f.txt", "1\tone"},
		{"tail -n 2 f.txt", "3\tthree\n4\tfour"},
		{"cat g.txt", "1\ta\n2\tb"},
		{"cat g.txt g.txt", "==> " + filepath.Join(dir, "g.txt") + " <==\n1\ta\n2\tb\n\n==> " + filepath.Join(dir, "g.txt") + " <==\n1\ta\n2\tb"},
	}
	for _, tc := range cases {
		updated, note := postOutput(t, RunPost(postPayload(t, tc.cmd, dir, ok)))
		assert.Equal(t, map[string]any{"stdout": tc.stdout, "stderr": "", "interrupted": false}, updated, tc.cmd)
		assert.Contains(t, note, "Your Bash command `"+tc.cmd+"` read a file", tc.cmd)
	}
}

func TestRunPostLeavesEverythingElseAlone(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644))
	ok := map[string]any{"stdout": "x\n", "stderr": "", "interrupted": false}
	for name, in := range map[string]*strings.Reader{
		"unmapped":    postPayload(t, "cat f.txt | wc -l", dir, ok),
		"stderr":      postPayload(t, "cat f.txt", dir, map[string]any{"stdout": "", "stderr": "boom", "interrupted": false}),
		"interrupted": postPayload(t, "cat f.txt", dir, map[string]any{"stdout": "", "stderr": "", "interrupted": true}),
		"missing":     postPayload(t, "cat nope.txt", dir, ok),
		"other tool":  strings.NewReader(`{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{}}`),
		"pre event":   strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"cat f.txt"}}`),
		"garbage":     strings.NewReader("not json"),
	} {
		assert.Equal(t, HookResult{}, RunPost(in), name)
	}
}

// A read that PlanRead maps runs. One it does not map keeps the deny.
func TestAMappedReadIsNotDenied(t *testing.T) {
	dir := t.TempDir()
	for _, cmd := range []string{"sed -n 2,3p f.txt", "cat f.txt"} {
		got := TransformIn(cmd, dir)
		assert.False(t, got.Denied, cmd)
		assert.False(t, got.Changed, "%s: a rewrite makes it a second statement", cmd)
	}
	assert.True(t, TransformIn("cat f.txt | jq .x", dir).Denied)
	assert.True(t, TransformIn("cat f.txt", "").Denied)
}
