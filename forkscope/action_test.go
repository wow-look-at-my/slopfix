package forkscope

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitUsesCutsEachPart(t *testing.T) {
	cases := map[string][4]string{
		"owner/repo@v1":          {"owner/repo", "", "v1", "true"},
		"owner/repo/sub/dir@abc": {"owner/repo", "sub/dir", "abc", "true"},
		"owner/repo":             {"", "", "", "false"},
		"owner@v1":               {"", "", "", "false"},
		"owner/repo@":            {"", "", "", "false"},
	}
	for uses, want := range cases {
		repo, path, ref, ok := splitUses(uses)
		assert.Equal(t, want, [4]string{repo, path, ref, map[bool]string{true: "true", false: "false"}[ok]}, uses)
	}
}
