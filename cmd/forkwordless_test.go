package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
)

// parentDebugDoc is the parent's module doc of a Rust file.
const parentDebugDoc = "//! `/debug`: debug-overlay toggles (scroll HUD, FPS HUD, scroll log).\n" +
	"//!\n" +
	"//! The command is registered on every binary and fully functional in release.\n" +
	"//! It is listed only on debug binaries.\n" +
	"//!\n" +
	"//! Subcommands:\n" +
	"//! - `/debug` bare: print the toggles and their state to the transcript.\n" +
	"//! - `/debug fps`: the release-safe FPS HUD.\n" +
	"\nuse std::path::Path;\n"

// forkDebugDoc rewrites that doc past the volume cap. Its blank lines match the parent's blank lines.
const forkDebugDoc = "//! `/debug <what is wrong>` hands the model this process's context.\n" +
	"//!\n" +
	"//! `/debug why was the context size defaulted?` injects the question too.\n" +
	"//!\n" +
	"//! - The debug-log file the firehose writes for this session. `/debug` turns\n" +
	"//!   the firehose on first, in this process and in the agent process.\n" +
	"//!   It is created if it does not exist.\n" +
	"//! - Whether the firehose ran since launch or only since this `/debug`.\n" +
	"//! - The rest of the execution context, assembled by the context module.\n" +
	"//!\n" +
	"//! Delivery is the inject path, the same path skills and `/loop`\n" +
	"//! use, so the prompt reaches the model as the next turn's content.\n" +
	"//!\n" +
	"//! Args that are not a reserved keyword are the user's question.\n" +
	"//! - `/debug scroll` toggles the scroll-diagnostics HUD.\n" +
	"//! - `/debug fps` toggles the release-safe FPS HUD.\n" +
	"//! - `/debug log` toggles the scroll flight recorder.\n" +
	"\nuse std::path::Path;\n"

// A tree fix in a fork reads the fork's lines from git.
func TestFixOfAForkTreeCutsADocTheForkMadeTooLong(t *testing.T) {
	fx := newForkRepoWith(t, aFork, map[string]string{"debug.rs": parentDebugDoc})
	put(t, fx.dir, "debug.rs", forkDebugDoc)
	gitRun(t, fx.dir, "add", "-A")
	gitRun(t, fx.dir, "commit", "-q", "-m", "a longer debug doc")

	cmd, out := quietCmd()
	_, err := treeFindings(cmd, fx.dir, slopfix.Request{}, true, fx.forks, nil)
	require.NoError(t, err)
	got := readT(t, filepath.Join(fx.dir, "debug.rs"))
	for _, line := range strings.Split(got, "\n") {
		assert.NotContains(t, line, ". - ", "a fold joined one list item onto another: %q", line)
	}

	cmd, out = quietCmd()
	failed, err := treeFindings(cmd, fx.dir, slopfix.Request{}, false, fx.forks, nil)
	require.NoError(t, err)
	assert.False(t, failed, "the check after the fix still fails:\n%s\n%s", out.String(), got)
}
