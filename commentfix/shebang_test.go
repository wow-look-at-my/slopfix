package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A shebang is the kernel's line, not prose. A grammar reads it as a comment
// because it opens on the marker, so a repair that reflows the run beneath it
// welds the first sentence onto the interpreter and the file stops running.
func TestTheRepairLeavesAShebangAlone(t *testing.T) {
	src := "#!/usr/bin/env bash\n" +
		"# Provisions bubblewrap on a Linux runner. Two phases need it. The dats phase\n" +
		"# sandboxes every suite command. The go command confines the generate directive\n" +
		"# of every dependency that carries one, and stops the build when it cannot.\n" +
		"#\n" +
		"# So every Linux build needs a backend, not only a module with dats suites: a\n" +
		"# module's own tree says nothing about what its dependencies generate.\n" +
		"#\n" +
		"# usage: provision-bwrap.sh\n" +
		"set -euo pipefail\n"

	out := Fix("provision-bwrap.sh", src)
	t.Logf("changed=%v removed=%v\n%s", out.Changed, out.Removed, out.Text)

	first, _, _ := strings.Cut(out.Text, "\n")
	assert.Equal(t, "#!/usr/bin/env bash", first, "the shebang keeps its own line")
	require.Contains(t, out.Text, "set -euo pipefail")
}

// The same line in a file the walk reaches by extension rather than by shape.
func TestTheRepairLeavesAShebangAloneInEveryShell(t *testing.T) {
	for _, name := range []string{"run.sh", "run.bash", "run.py"} {
		src := "#!/bin/sh\n# Two phases need it here.\nexit 0\n"
		first, _, _ := strings.Cut(Fix(name, src).Text, "\n")
		assert.Equal(t, "#!/bin/sh", first, name)
	}
}
