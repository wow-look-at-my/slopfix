package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

// wrapped is a hard-wrapped paragraph, which wrap/hard-wrap reports in any document.
const wrapped = "# Prompt\n\nThe agent reads the file\nand writes the result.\n"

func TestAFileCodeEmbedsIsNeitherJudgedNorRewritten(t *testing.T) {
	top := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = top
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(top, "src"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(top, "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(top, "src", "lib.rs"), []byte("const P: &str = include_str!(\"../templates/prompt.md\");\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(top, "templates", "prompt.md"), []byte(wrapped), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(top, "templates", "notes.md"), []byte(wrapped), 0o644))
	run("init", "-q")
	run("add", "-A")

	prompt := filepath.Join(top, "templates", "prompt.md")
	assert.True(t, slopfix.Embedded(prompt))
	assert.Empty(t, slopfix.Report(slopfix.Request{Path: prompt, Content: wrapped}).Findings)
	repair := slopfix.Fix(slopfix.Request{Path: prompt, Content: wrapped})
	assert.Equal(t, wrapped, repair.Text, "the embedded prompt keeps every byte")
	assert.False(t, repair.Changed)

	notes := filepath.Join(top, "templates", "notes.md")
	assert.False(t, slopfix.Embedded(notes))
	assert.NotEmpty(t, slopfix.Report(slopfix.Request{Path: notes, Content: wrapped}).Findings, "a file nothing embeds is still judged")
}
