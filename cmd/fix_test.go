package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

func TestACategoryTurnsItsWholeRuleSetOn(t *testing.T) {
	rules, ids, err := selectedRules([]string{"counts", " ste "})
	require.NoError(t, err)

	assert.Equal(t, []slopfix.Rule{slopfix.RuleCounts, slopfix.RuleSTE}, rules)
	assert.Empty(t, ids)
}

// An ID turns its category on too, so naming a rule never needs the category
// named beside it.
func TestARuleIDTurnsItsCategoryOn(t *testing.T) {
	rules, ids, err := selectedRules([]string{"ste/semicolon"})
	require.NoError(t, err)

	assert.Equal(t, []slopfix.Rule{slopfix.RuleSTE}, rules)
	assert.Equal(t, []string{"ste/semicolon"}, ids)
}

// The selection reaches a named file, not only a document on stdin. A run that
// named a rule and got another rule's whole-line strip has lost prose the
// caller never offered up.
func TestTheSelectionReachesANamedFile(t *testing.T) {
	src := "// Add inserts elem. It returns true when the element was added,\n" +
		"// or false when it was already present.\nfunc Add() {}\n\n" +
		"// It reserves one slot.\nfunc B() {}\n"
	path := filepath.Join(t.TempDir(), "set.go")
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))

	rules, ids, err := selectedRules([]string{"comments/number"})
	require.NoError(t, err)
	cmd := &cobra.Command{}
	require.NoError(t, fixFiles(cmd, []string{path}, rules, ids))

	repaired, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(repaired), "// Add inserts elem. It returns true when the element was added,",
		"the number repair does not delete another rule's line")
	assert.Contains(t, string(repaired), "// It reserves a single slot.")
}

// A typo is an error rather than a silent no-op, because a run that applies
// nothing reads as a clean file.
func TestATypoIsAnError(t *testing.T) {
	_, _, err := selectedRules([]string{"ste/nosuch"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ste/semicolon")

	_, _, err = selectedRules([]string{"nosuch"})
	require.Error(t, err)

	_, _, err = selectedRules([]string{"nosuch/rule"})
	require.Error(t, err)
}
