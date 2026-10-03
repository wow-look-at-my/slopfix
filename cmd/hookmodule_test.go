package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The embedded module runs `check read-plan`, and the plugin build runs
// `hook module`. Both must stay registered where those callers look.
func TestTheModuleCommandsAreRegistered(t *testing.T) {
	for _, path := range [][]string{{"check", "read-plan"}, {"hook", "module"}} {
		c, rest, err := rootCmd.Find(path)
		require.NoError(t, err, "%v", path)
		assert.Empty(t, rest, "%v", path)
		assert.Equal(t, path[1], c.Name(), "%v", path)
	}
}
