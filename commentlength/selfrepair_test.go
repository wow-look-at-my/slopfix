package commentlength

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// This repository's own source is the rule's first real corpus, and running the
// rule over it is the only evidence that the repair works on prose somebody
// wrote for its own sake rather than for a fixture.
//
// It reports by default and never writes. Set SLOPFIX_SELF_REPAIR=1 to apply,
// which is how the tree gets cleaned once: the binary that carries this rule
// cannot be built while go-toolchain's own commentspan warnings hold the build,
// so the first repair has to come from the test run that precedes the gate.
func TestTheRuleOverItsOwnRepository(t *testing.T) {
	root := ".."
	apply := os.Getenv("SLOPFIX_SELF_REPAIR") == "1"

	var files, findings, repaired int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "build", "bin", "grammars", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !Parsed(path) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		files++
		hits := Check(path, string(src))
		findings += len(hits)
		if !apply || len(hits) == 0 {
			return nil
		}
		out, changed := Fix(path, string(src))
		if !changed {
			return nil
		}
		require.NoError(t, os.WriteFile(path, []byte(out), info.Mode().Perm()))
		repaired++
		return nil
	})
	require.NoError(t, err)
	require.NotZero(t, files, "the walk found no parsable file, so it proved nothing")

	t.Logf("files parsed: %d", files)
	t.Logf("findings:     %d", findings)
	if apply {
		t.Logf("files repaired: %d", repaired)
	}
}

// The walk above must not be fooled by a path it cannot parse. This pins that
// Parsed() and the extension map agree about what the rule reads.
func TestParsedAgreesWithTheGrammarMap(t *testing.T) {
	for ext := range grammars {
		require.True(t, Parsed("x"+ext), "the map has %s but Parsed says no", ext)
		require.True(t, Parsed("x"+strings.ToUpper(ext)), "extension match is case-insensitive")
	}
	require.False(t, Parsed("x.md"))
	require.False(t, Parsed("x"))
}
