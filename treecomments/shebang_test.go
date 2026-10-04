package treecomments

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A grammar reads the interpreter line as a comment, because it opens on the
// same marker. Nothing downstream may see it: a rule that reflows a run would
// weld the prose under it onto the interpreter, and the kernel then has no
// program to start.
func TestTheShebangIsNotAComment(t *testing.T) {
	src := "#!/usr/bin/env bash\n# what this script does\nexit 0\n"

	got := Extract("run.sh", src)
	require.Len(t, got, 1)
	assert.Equal(t, "# what this script does", got[0].Text)
	assert.Equal(t, 2, got[0].Line)
}

// The run beneath a shebang stands on its own, so a reflow rewrites it without
// reaching the line above.
func TestAShebangStartsNoRun(t *testing.T) {
	src := "#!/bin/sh\n# first\n# second\nexit 0\n"

	runs := Runs("run.sh", src)
	require.Len(t, runs, 1)
	assert.Len(t, runs[0], 2)
	assert.Equal(t, 2, runs[0][0].Line)
}

// A `#!` that is not the top line at the left edge is prose somebody wrote.
func TestAMarkerBelowTheTopLineStaysAComment(t *testing.T) {
	src := "# a note\n# on `#!` lines\nexit 0\n"

	assert.Len(t, Extract("run.sh", src), 2)
}
