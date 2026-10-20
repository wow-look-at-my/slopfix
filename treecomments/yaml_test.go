package treecomments

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/yaml-inner-hash.yml
var yamlInnerHash string

func TestAWorkflowCommentStartsAtItsOwnMarker(t *testing.T) {
	var got []Comment
	for _, c := range Extract(".github/workflows/ci.yml", yamlInnerHash) {
		if c.Line == 4 {
			got = append(got, c)
		}
	}
	require.Len(t, got, 1)
	assert.Equal(t, 2, got[0].Col)
	assert.Equal(t, "# The tool's table and every `#[cfg(test)]` module stay out.", got[0].Text)
}

func TestTheBashGrammarAloneMisreadsTheWorkflowComment(t *testing.T) {
	language, _ := grammarFor(".github/workflows/ci.yml")
	require.NotNil(t, language)
	misread := false
	for _, c := range extract(language, yamlInnerHash) {
		if c.Line == 4 && c.Col > 2 {
			misread = true
		}
	}
	assert.True(t, misread, "the fixture must reproduce the misread, or the test above proves nothing")
}
