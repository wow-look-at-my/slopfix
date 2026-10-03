package bashclean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCalls(t *testing.T) {
	dir := t.TempDir()
	body := strings.Repeat("line\n", 99) + "last"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte(body), 0o644))
	f := filepath.Join(dir, "f.txt")
	q := `"` + f + `"`
	cases := []struct{ name, cmd, want string }{
		{"cat", "cat f.txt", "Read(file_path=" + q + ")"},
		{"cat -n", "cat -n f.txt", "Read(file_path=" + q + ")"},
		{"cat of two files", "cat f.txt f.txt", "Read(file_path=" + q + "); Read(file_path=" + q + ")"},
		{"absolute path", "cat " + f, "Read(file_path=" + q + ")"},
		{"head default", "head f.txt", "Read(file_path=" + q + ", limit=10)"},
		{"head -n N", "head -n 20 f.txt", "Read(file_path=" + q + ", limit=20)"},
		{"head -nN", "head -n20 f.txt", "Read(file_path=" + q + ", limit=20)"},
		{"head -N", "head -60 f.txt", "Read(file_path=" + q + ", limit=60)"},
		{"head --lines=N", "head --lines=5 f.txt", "Read(file_path=" + q + ", limit=5)"},
		{"head --lines N", "head --lines 5 f.txt", "Read(file_path=" + q + ", limit=5)"},
		{"tail counts from the end", "tail -n 20 f.txt", "Read(file_path=" + q + ", offset=81, limit=20)"},
		{"tail default", "tail f.txt", "Read(file_path=" + q + ", offset=91, limit=10)"},
		{"tail past the start", "tail -500 f.txt", "Read(file_path=" + q + ", offset=1, limit=100)"},
		{"tail +K starts at line K", "tail -n +40 f.txt", "Read(file_path=" + q + ", offset=40)"},
		{"sed range", "sed -n '10,20p' f.txt", "Read(file_path=" + q + ", offset=10, limit=11)"},
		{"sed one line", "sed -n 5p f.txt", "Read(file_path=" + q + ", offset=5, limit=1)"},
		{"sed --quiet", "sed --quiet '3,4p' f.txt", "Read(file_path=" + q + ", offset=3, limit=2)"},
		{"command wrapper", "command head -3 f.txt", "Read(file_path=" + q + ", limit=3)"},
	}
	for _, tc := range cases {
		got := TransformIn(tc.cmd, dir)
		require.True(t, got.Denied, "%s: expected a deny", tc.name)
		assert.Equal(t, tc.want, strings.Join(got.ReadCalls, "; "), "%s", tc.name)
		assert.Contains(t, fullDenyReason(got), "Make this call instead: "+tc.want, "%s", tc.name)
	}
}

// Each of these is still denied, but no single Read call prints the same
// thing, so the deny must name none.
func TestReadCallsWithNoExactEquivalent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\nb\n"), 0o644))
	for _, cmd := range []string{
		"cat f.txt | jq .x",
		"cat f.txt > g.txt",
		"cd sub && head -5 f.txt",
		"cat f.txt\nmake",
		"tail -f f.txt",
		"tail -n 5 missing.txt",
		"head -c 20 f.txt",
		"head -n 0 f.txt",
		"head -n -5 f.txt",
		"sed -n '/re/p' f.txt",
		"sed -n '5,2p' f.txt",
		"sed -n -e 3p f.txt",
		"cat -A f.txt",
	} {
		got := TransformIn(cmd, dir)
		require.True(t, got.Denied, "%q: expected a deny", cmd)
		assert.Empty(t, got.ReadCalls, "%q", cmd)
		assert.Equal(t, denyReason("file_read"), fullDenyReason(got), "%q", cmd)
	}
}

func TestReadCallsNeedAnAbsolutePath(t *testing.T) {
	got := Transform("head -5 f.txt")
	require.True(t, got.Denied)
	assert.Empty(t, got.ReadCalls)
}

func TestRunNamesTheReadCall(t *testing.T) {
	dir := t.TempDir()
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":` + `"` + dir + `"` +
		`,"tool_input":{"command":"sed -n '2,3p' notes.md"}}`
	out := Run(strings.NewReader(payload))
	assert.Contains(t, out.Stdout, `"permissionDecision":"deny"`)
	assert.Contains(t, out.Stdout, `Read(file_path=\"`+filepath.Join(dir, "notes.md")+`\", offset=2, limit=2)`)
}
