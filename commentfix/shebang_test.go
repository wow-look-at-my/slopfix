package commentfix

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A shebang is the kernel's line, not prose. A grammar reads it as a comment.
// This is because it opens on the marker, so a repair that reflows the run
// beneath it welds the earliest sentence onto the interpreter. The file stops
// running.
func TestTheRepairLeavesAShebangAlone(t *testing.T) {
	src := testTheRepairLeavesAShebangAloneSh

	out := Fix("provision-bwrap.sh", src)
	t.Logf("changed=%v removed=%v\n%s", out.Changed, out.Removed, out.Text)

	first, _, _ := strings.Cut(out.Text, "\n")
	assert.Equal(t, "#!/usr/bin/env bash", first, "the shebang keeps its own line")
	require.Contains(t, out.Text, "set -euo pipefail")
}

// The same line in a file the walk reaches by extension rather than by shape.
func TestTheRepairLeavesAShebangAloneInEveryShell(t *testing.T) {
	for _, name := range []string{"run.sh", "run.bash", "run.py"} {
		src := testTheRepairLeavesAShebangAloneInEveryShellSh
		first, _, _ := strings.Cut(Fix(name, src).Text, "\n")
		assert.Equal(t, "#!/bin/sh", first, name)
	}
}

//go:embed testdata/test-the-repair-leaves-a-shebang-alone.sh
var testTheRepairLeavesAShebangAloneSh string

//go:embed testdata/test-the-repair-leaves-a-shebang-alone-in-every-shell.sh
var testTheRepairLeavesAShebangAloneInEveryShellSh string
