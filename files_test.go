package slopfix_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestCheckFileReportsWhatTheDocumentBreaks(t *testing.T) {
	path := write(t, "a.md", "The loader doesn't read the flag.\n")

	findings, err := slopfix.CheckFile(path)
	require.NoError(t, err)
	assert.NotEmpty(t, findings)
}

func TestCheckFileSaysSoWhenTheFileIsMissing(t *testing.T) {
	_, err := slopfix.CheckFile(filepath.Join(t.TempDir(), "absent.md"))
	assert.Error(t, err)
}

func TestFormatFileJoinsAWrappedParagraphInPlace(t *testing.T) {
	const wrapped = "The loader reads\nthe flag it names.\n"
	path := write(t, "a.md", wrapped)

	changed, err := slopfix.FormatFile(path)
	require.NoError(t, err)
	assert.True(t, changed)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	// The join moves newlines and nothing else: the same words, on a single line.
	assert.Equal(t, strings.Fields(wrapped), strings.Fields(string(after)))
	assert.Equal(t, 1, strings.Count(string(after), "\n"))
}

func TestFormatFileLeavesAFormattedDocumentAlone(t *testing.T) {
	path := write(t, "a.md", "The loader reads the flag it names.\n")

	changed, err := slopfix.FormatFile(path)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestFormatFileSaysSoWhenTheFileIsMissing(t *testing.T) {
	_, err := slopfix.FormatFile(filepath.Join(t.TempDir(), "absent.md"))
	assert.Error(t, err)
}
