package slopfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfmt"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestCheckFileReportsWhatTheDocumentBreaks(t *testing.T) {
	path := write(t, "a.md", "The loader doesn't read the flag.\n")

	findings, err := slopfmt.CheckFile(path)
	require.NoError(t, err)
	assert.NotEmpty(t, findings)
}

func TestCheckFileSaysSoWhenTheFileIsMissing(t *testing.T) {
	_, err := slopfmt.CheckFile(filepath.Join(t.TempDir(), "absent.md"))
	assert.Error(t, err)
}

func TestFormatFileJoinsAWrappedParagraphInPlace(t *testing.T) {
	path := write(t, "a.md", "The loader reads\nthe flag it names.\n")

	changed, err := slopfmt.FormatFile(path)
	require.NoError(t, err)
	assert.True(t, changed)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "The loader reads the flag it names.\n", string(after))
}

func TestFormatFileLeavesAFormattedDocumentAlone(t *testing.T) {
	path := write(t, "a.md", "The loader reads the flag it names.\n")

	changed, err := slopfmt.FormatFile(path)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestFormatFileSaysSoWhenTheFileIsMissing(t *testing.T) {
	_, err := slopfmt.FormatFile(filepath.Join(t.TempDir(), "absent.md"))
	assert.Error(t, err)
}
