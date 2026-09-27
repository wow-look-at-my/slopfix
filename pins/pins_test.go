package pins

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/edit"
)

func TestVersionParameterIsAFinding(t *testing.T) {
	for _, line := range []string{
		"curl -fsSL 'https://dl.pazer.build/ts0?v=10&os=linux&arch=amd64' -o ts0.cjs",
		"curl -fsSL https://dl.pazer.build/ts0?os=linux&arch=amd64&v=10",
		"[download](https://dl.pazer.build/dats?os=linux&v=3&arch=amd64)",
		`<a href="https://dl.pazer.build/dats?os=linux&amp;v=3">dats</a>`,
		"curl https://dl.pazer.build/tool?os=${OS}&v=${VERSION}&arch=${ARCH}",
		"dl.pazer.build/tool?v",
	} {
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
		"https://static.pazer.build/file?project=ts0&v=74",
		"the page names v=10 as an example",
	} {
		assert.Empty(t, Check(line), line)
	}
}

func TestFindingNamesTheLineAndTheURL(t *testing.T) {
	text := "#!/bin/sh\nset -eu\ncurl -fsSL 'https://dl.pazer.build/ts0?v=10&os=linux' -o x\n"
	findings := Check(text)
	require.Len(t, findings, 1)
	assert.Equal(t, 3, findings[0].Line)
	assert.Equal(t, "dl.pazer.build/ts0?v=10&os=linux", findings[0].Detail)
}

func TestEveryPinnedURLOnALineIsReported(t *testing.T) {
	line := "a https://dl.pazer.build/a?v=1 and https://dl.pazer.build/b?os=linux&v=2"
	assert.Len(t, Check(line), 2)
}

func TestFragmentIsNotAParameter(t *testing.T) {
	assert.Empty(t, Check("https://dl.pazer.build/tool?os=linux#v=1"))
}

func repaired(text string) string {
	return Gate(text, Edits(text), edit.Scope{}).Text
}

func TestRepairDropsTheParameter(t *testing.T) {
	cases := map[string]string{
		"curl 'https://dl.pazer.build/ts0?v=10&os=linux&arch=amd64' -o x": "curl 'https://dl.pazer.build/ts0?arch=amd64&os=linux' -o x",
		"https://dl.pazer.build/ts0?os=linux&v=10":                          "https://dl.pazer.build/ts0?os=linux",
		"https://dl.pazer.build/ts0?v=10":                                   "https://dl.pazer.build/ts0",
		"https://dl.pazer.build/a?v=1#top":                                  "https://dl.pazer.build/a#top",
		"https://dl.pazer.build/a?v=1&v=2&os=linux":                         "https://dl.pazer.build/a?os=linux",
	}
	for in, want := range cases {
		assert.Equal(t, want, repaired(in), in)
		assert.Empty(t, Check(repaired(in)), in)
	}
}

// Encode would escape a template and rewrite an HTML-escaped separator, so
// such a URL is reported and left for a person.
func TestRepairLeavesAURLEncodeWouldMangle(t *testing.T) {
	for _, in := range []string{
		"curl https://dl.pazer.build/t?os=${OS}&v=${V}&arch=${ARCH}",
		`<a href="https://dl.pazer.build/a?os=linux&amp;v=3">a</a>`,
	} {
		assert.Equal(t, in, repaired(in), in)
		assert.Len(t, Check(in), 1, in)
	}
}

func TestRepairLeavesEverythingElse(t *testing.T) {
	text := "#!/bin/sh\nset -eu\ncurl -fsSL 'https://dl.pazer.build/ts0?v=10&os=linux' -o a\ncurl -fsSL 'https://dl.pazer.build/dats?os=linux' -o b\n"
	want := "#!/bin/sh\nset -eu\ncurl -fsSL 'https://dl.pazer.build/ts0?os=linux' -o a\ncurl -fsSL 'https://dl.pazer.build/dats?os=linux' -o b\n"
	assert.Equal(t, want, repaired(text))
}

func TestGateRefusesAnyOtherEdit(t *testing.T) {
	text := "https://dl.pazer.build/ts0?os=linux&v=10"
	res := Gate(text, []edit.Edit{{Start: 27, End: 36}}, edit.Scope{})
	assert.Equal(t, text, res.Text)
	require.Len(t, res.Refused, 1)
}
