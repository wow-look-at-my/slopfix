package bashclean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanRead(t *testing.T) {
	dir := t.TempDir()
	body := strings.Repeat("line\n", 29) + "last"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte(body), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "g.txt"), []byte("a\nb\nc\n"), 0o644))
	f := filepath.Join(dir, "f.txt")
	g := filepath.Join(dir, "g.txt")
	cases := []struct {
		cmd  string
		want []ReadArgs
	}{
		{"cat f.txt", []ReadArgs{{FilePath: f}}},
		{"cat -n " + f, []ReadArgs{{FilePath: f}}},
		{"command cat f.txt", []ReadArgs{{FilePath: f}}},
		{"cat f.txt g.txt", []ReadArgs{{FilePath: f}, {FilePath: g}}},
		{"cat 'f.txt'", []ReadArgs{{FilePath: f}}},
		{"head f.txt", []ReadArgs{{FilePath: f, Limit: 10}}},
		{"head -n 5 f.txt", []ReadArgs{{FilePath: f, Limit: 5}}},
		{"head -n5 f.txt", []ReadArgs{{FilePath: f, Limit: 5}}},
		{"head -60 f.txt", []ReadArgs{{FilePath: f, Limit: 60}}},
		{"head --lines=7 f.txt", []ReadArgs{{FilePath: f, Limit: 7}}},
		{"head -n 3 f.txt g.txt", []ReadArgs{{FilePath: f, Limit: 3}, {FilePath: g, Limit: 3}}},
		{"tail -n 5 f.txt", []ReadArgs{{FilePath: f, Offset: 26, Limit: 5}}},
		{"tail f.txt", []ReadArgs{{FilePath: f, Offset: 21, Limit: 10}}},
		{"tail -100 g.txt", []ReadArgs{{FilePath: g, Offset: 1, Limit: 3}}},
		{"tail -n +12 f.txt", []ReadArgs{{FilePath: f, Offset: 12}}},
		{"sed -n '10,20p' f.txt", []ReadArgs{{FilePath: f, Offset: 10, Limit: 11}}},
		{"sed -n 4p f.txt", []ReadArgs{{FilePath: f, Offset: 4, Limit: 1}}},
		{"sed --quiet 2,3p ./sub/../f.txt", []ReadArgs{{FilePath: f, Offset: 2, Limit: 2}}},
	}
	for _, tc := range cases {
		got := PlanRead(tc.cmd, dir)
		if !assert.NotNil(t, got, "%q: expected a plan", tc.cmd) {
			continue
		}
		assert.Equal(t, tc.want, got.Reads, "%q", tc.cmd)
		assert.Contains(t, got.Note, "Your Bash command `"+tc.cmd+"` read a file, so its output is shown as the Read tool shows it.", "%q", tc.cmd)
	}
}

func TestPlanReadNote(t *testing.T) {
	got := PlanRead("sed -n '2,3p' a.txt", "/work")
	require.NotNil(t, got)
	assert.Equal(t, "Your Bash command `sed -n '2,3p' a.txt` read a file, so its output is shown as the Read tool shows it. These Read calls give the same lines:\n"+
		"- Read(file_path: \"/work/a.txt\", offset: 2, limit: 2)\n"+
		"Use the Read tool to read files. Its offset and limit parameters select lines, which is what head, tail and sed -n were for.",
		got.Note)
}

// Each of these prints something no set of Read calls prints, so the command
// must run as written.
func TestPlanReadLeavesTheRestToBash(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n"), 0o644))
	for _, cmd := range []string{
		"cat f.txt | jq .x",
		"cat f.txt > g.txt",
		"cd sub && cat f.txt",
		"cat f.txt; make",
		"X=1 cat f.txt",
		"cat $F",
		"cat *.txt",
		"cat \"$HOME/f\"",
		"cat -A f.txt",
		"cat",
		"cat -",
		"cat /proc/meminfo",
		"head -c 20 f.txt",
		"head -n 0 f.txt",
		"head -n -5 f.txt",
		"head -n +5 f.txt",
		"tail -f f.txt",
		"tail -n 5 missing.txt",
		"sed -n '/re/p' f.txt",
		"sed -n '5,2p' f.txt",
		"sed -n 0p f.txt",
		"sed -n -e 3p f.txt",
		"sed -n 2p f.txt g.txt",
		"sed 3p f.txt",
		"sed 's/a/b/' f.txt",
		"ls -la",
		"if true; then",
	} {
		assert.Nil(t, PlanRead(cmd, dir), "%q", cmd)
	}
}

func TestPlanReadNeedsAnAbsoluteDirectory(t *testing.T) {
	assert.Nil(t, PlanRead("cat f.txt", ""))
	assert.Nil(t, PlanRead("cat f.txt", "rel"))
	assert.NotNil(t, PlanRead("cat /abs/f.txt", ""))
}
