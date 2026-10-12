package cmd

import (
	"bytes"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const commit = "660ecca3f00d8e1b2c3d4e5f60718293a4b5c6d7"

// runRoot drives the root command over build info that carries the given
// release settings (an empty value is left out), returning stdout and the error.
func runRoot(t *testing.T, number, rev string, args ...string) (string, error) {
	t.Helper()
	cmdMu.Lock()
	defer cmdMu.Unlock()
	settings := []debug.BuildSetting{{Key: "-compiler", Value: "gc"}}
	if number != "" {
		settings = append(settings, debug.BuildSetting{Key: "release", Value: number})
	}
	if rev != "" {
		settings = append(settings, debug.BuildSetting{Key: "release.revision", Value: rev})
	}
	read := buildSettings
	buildSettings = func() []debug.BuildSetting { return settings }
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs(args)
	defer func() {
		buildSettings = read
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
		require.NoError(t, rootCmd.Flags().Set("version", "false"))
	}()
	err := rootCmd.Execute()
	return out.String(), err
}

func TestVersionFlagPrintsTheReleaseNumberAndCommit(t *testing.T) {
	out, err := runRoot(t, "1234", commit, "--version")
	require.NoError(t, err)
	assert.Equal(t, "slopfix 1234 ("+commit+")\n", out)
}

func TestVersionCommandPrintsTheReleaseNumberAndCommit(t *testing.T) {
	out, err := runRoot(t, "1234", commit, "version")
	require.NoError(t, err)
	assert.Equal(t, "slopfix 1234 ("+commit+")\n", out)
}

func TestAMissingReleaseNumberFailsInsteadOfNamingAVersion(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"version"}} {
		out, err := runRoot(t, "", commit, args...)
		require.ErrorIs(t, err, errNoRelease, args)
		assert.Empty(t, out, args)
	}
}

func TestAMissingCommitFailsInsteadOfNamingAVersion(t *testing.T) {
	out, err := runRoot(t, "1234", "", "version")
	require.ErrorIs(t, err, errNoRevision)
	assert.Empty(t, out)
}

func TestBareRootStillPrintsHelp(t *testing.T) {
	out, err := runRoot(t, "1234", commit)
	require.NoError(t, err)
	assert.Contains(t, out, "Usage:")
}
