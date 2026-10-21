package forkscope

import (
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertedLinesNamesWhatADiffAdds(t *testing.T) {
	cases := []struct {
		before, after string
		want          []int
	}{
		{"", "a\nb\n", []int{1, 2}},
		{"a\nb\n", "", nil},
		{"a\nb\nc\n", "a\nb\nc\n", nil},
		{"a\nb\nc\n", "a\nx\nc\n", []int{2}},
		{"a\nc\n", "a\nb\nc\nd\n", []int{2, 4}},
		{"a\nb", "a\nb\n", []int{2}},
		{"one\ntwo\nthree\n", "zero\none\nthree\nfour\n", []int{1, 4}},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, insertedLines(c.before, c.after), "%q to %q", c.before, c.after)
	}
}

// The diff is as short as git's: on generated edits it adds as many lines as
// git diff --numstat counts.
func TestInsertedLinesAddAsManyLinesAsGit(t *testing.T) {
	dir := t.TempDir()
	rng := rand.New(rand.NewSource(1))
	words := []string{"alpha\n", "beta\n", "gamma\n", "delta\n", "\n", "}\n"}
	text := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteString(words[rng.Intn(len(words))])
		}
		return b.String()
	}
	for i := 0; i < 40; i++ {
		before, after := text(rng.Intn(60)), text(rng.Intn(60))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte(before), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "b"), []byte(after), 0o644))
		cmd := exec.Command("git", "diff", "--no-index", "--numstat", "--minimal", "a", "b")
		cmd.Dir = dir
		out, _ := cmd.Output()
		want := 0
		if fields := strings.Fields(string(out)); len(fields) > 0 {
			want, _ = strconv.Atoi(fields[0])
		}
		assert.Len(t, insertedLines(before, after), want, "case %d", i)
	}
}
