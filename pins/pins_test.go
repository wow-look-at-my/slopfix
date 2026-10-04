package pins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/edit"
)

// The pinned fixtures live in testdata, which the walk skips, so this
// repository's own check does not report them.
func fixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(body)
}

func lines(t *testing.T, name string) []string {
	return strings.Split(strings.TrimSuffix(fixture(t, name), "\n"), "\n")
}

func repaired(text string) string {
	return Gate(text, Edits(text), edit.Scope{}).Text
}

func TestVersionParameterIsAFinding(t *testing.T) {
	for _, line := range lines(t, "detect.urls") {
		findings := Check(line)
		require.Len(t, findings, 1, line)
		assert.Equal(t, ID, findings[0].ID)
		assert.Equal(t, 1, findings[0].Line)
	}
}

func TestUnversionedURLIsClean(t *testing.T) {
	for _, line := range []string{
		"curl -fsSL 'https://dl.pazer.build/ts0?os=linux&arch=amd64' -o ts0.cjs",
		"curl -fsSL 'https://dl.pazer.build/ts0?branch=master&os=linux&arch=amd64'",
		"https://dl.pazer.build/dats",
		"https://dl.pazer.build/tool?version_hint=v&vv=1",
		"https://dl.pazer.build/tool?os=linux#v=1",
		"https://static.pazer.build/file?project=ts0&v=74",
		"the page names v=10 as an example",
	} {
		assert.Empty(t, Check(line), line)
	}
}

func TestFindingNamesTheLineAndTheURL(t *testing.T) {
	findings := Check(fixture(t, "fetch.sh"))
	require.Len(t, findings, 1)
	assert.Equal(t, 3, findings[0].Line)
	assert.True(t, strings.HasPrefix(findings[0].Detail, "dl.pazer.build/ts0?"), findings[0].Detail)
}

func TestEveryPinnedURLOnALineIsReported(t *testing.T) {
	assert.Len(t, Check(fixture(t, "twice.urls")), 2)
}

func TestRepairDropsTheParameter(t *testing.T) {
	for _, pair := range lines(t, "repair.urls") {
		in, want, found := strings.Cut(pair, "\t")
		require.True(t, found, pair)
		assert.Equal(t, want, repaired(in), in)
		assert.Empty(t, Check(repaired(in)), in)
	}
}

// Encode would escape a template and rewrite an HTML-escaped separator, so such
// a URL loses v as text and keeps every other byte.
func TestRepairKeepsATemplateAndAnEscapedSeparator(t *testing.T) {
	for _, pair := range lines(t, "mangle.urls") {
		in, want, found := strings.Cut(pair, "\t")
		require.True(t, found, pair)
		assert.Len(t, Check(in), 1, in)
		assert.Equal(t, want, repaired(in), in)
		assert.Empty(t, Check(repaired(in)), in)
	}
}

func TestRepairLeavesEverythingElse(t *testing.T) {
	assert.Equal(t, fixture(t, "fetch.fixed.sh"), repaired(fixture(t, "fetch.sh")))
}

func TestGateRefusesAnyOtherEdit(t *testing.T) {
	text := fixture(t, "fetch.sh")
	at := strings.Index(text, "os=linux")
	res := Gate(text, []edit.Edit{{Start: at, End: at + len("os=linux")}}, edit.Scope{})
	assert.Equal(t, text, res.Text)
	require.Len(t, res.Refused, 1)
}
