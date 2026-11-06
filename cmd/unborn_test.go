package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goneParent answers that o/fork is a fork of a parent no fetch can reach.
var goneParent = answers(func(string) any {
	return map[string]any{"fork": true, "parent": map[string]any{"clone_url": "/nonexistent/parent.git", "default_branch": "main"}}
})

// A write of a new file needs no history of the parent. The fork wrote every
// line of it, so the hook must not fetch, and must not fail when a fetch would.
func TestANewFileInAForkNeedsNoParent(t *testing.T) {
	fx := newForkRepo(t, goneParent)

	owned, err := forkLinesOf(fx.forks, filepath.Join(fx.dir, "incr-bench", "results.md"))
	require.NoError(t, err)
	scope, err := owned("# Results\n")
	require.NoError(t, err)
	require.NotNil(t, scope)
	assert.True(t, scope.All(), "a file new to the fork is the fork's whole")

	_, err = forkLinesOf(fx.forks, filepath.Join(fx.dir, "doc.md"))
	require.Error(t, err, "a tracked file still needs the base, so the parent here is truly out of reach")
	assert.Contains(t, err.Error(), "fetch the parent's main")
}
