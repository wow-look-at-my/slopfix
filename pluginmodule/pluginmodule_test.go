package pluginmodule

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The module asks this binary for the plan. The argv it runs must name the
// command cmd/readplan.go registers, or every Bash read runs as Bash.
func TestTheModuleCallsCheckReadPlan(t *testing.T) {
	assert.Contains(t, Source(), "slopfix.ape`, 'check', 'read-plan'")
}

func TestWriteLaysOutEveryFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hooks")
	require.NoError(t, Write(dir))
	for _, name := range Names {
		got, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		want, err := files.ReadFile(name)
		require.NoError(t, err)
		assert.Equal(t, string(want), string(got), name)
	}
}
