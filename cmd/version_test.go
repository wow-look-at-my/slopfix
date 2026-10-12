package cmd

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBuildSettingsStamped(t *testing.T) {
	b := parseBuildSettings([]debug.BuildSetting{
		{Key: "vcs.revision", Value: "660ecca3f00d"},
		{Key: "vcs.time", Value: "2026-10-12T04:31:00Z"},
		{Key: "vcs.modified", Value: "false"},
	})
	assert.Equal(t, "v0.0.1791779460", b.version())
	assert.Equal(t, "v0.0.1791779460 (660ecca3f00d, 2026-10-12T04:31:00Z)", b.String())
}

func TestParseBuildSettingsModified(t *testing.T) {
	b := parseBuildSettings([]debug.BuildSetting{
		{Key: "vcs.revision", Value: "abc"},
		{Key: "vcs.time", Value: "2026-10-12T04:31:00Z"},
		{Key: "vcs.modified", Value: "true"},
	})
	assert.True(t, strings.HasSuffix(b.String(), " modified"))
}

func TestParseBuildSettingsUnstamped(t *testing.T) {
	b := parseBuildSettings(nil)
	assert.Equal(t, "dev", b.version())
	assert.Equal(t, "dev", b.String())
}

func runRoot(t *testing.T, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	})
	require.NoError(t, rootCmd.Execute())
	return out.String()
}

func TestVersionFlag(t *testing.T) {
	assert.Equal(t, "slopfix "+readBuildInfo().String()+"\n", runRoot(t, "--version"))
}

func TestVersionCommand(t *testing.T) {
	assert.Equal(t, "slopfix "+readBuildInfo().String()+"\n", runRoot(t, "version"))
}
