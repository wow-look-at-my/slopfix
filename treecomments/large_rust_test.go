package treecomments

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractParsesALargeRustFile(t *testing.T) {
	src, err := os.ReadFile("testdata/goal_support_rs.txt")
	require.NoError(t, err)
	comments := Extract("goal_support.rs", string(src))
	require.NotEmpty(t, comments, "the file has doc comments; an empty result means the parse failed")
}
