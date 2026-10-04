package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

func scriptFindings(findings []slopfix.TreeFinding) []slopfix.TreeFinding {
	var out []slopfix.TreeFinding
	for _, f := range findings {
		if f.ID == slopfix.IDPackageScripts {
			out = append(out, f)
		}
	}
	return out
}

func TestAPackageJSONWithScriptsIsReported(t *testing.T) {
	root := gitRoot(t, map[string]string{
		"package.json":     "{\n\t\"name\": \"a\",\n\t\"scripts\": {\"build\": \"tsc\"}\n}\n",
		"sub/package.json": "{\"name\": \"b\", \"scripts\": {}}\n",
	})
	found := scriptFindings(slopfix.CheckTree(root).Findings)
	require.Len(t, found, 2)
	assert.Equal(t, "package.json", found[0].Path)
	assert.Equal(t, 3, found[0].Line)
	assert.Equal(t, "sub/package.json", found[1].Path)
	assert.Contains(t, found[0].Fix, "justfile")
}

func TestAPackageJSONWithoutScriptsPasses(t *testing.T) {
	root := gitRoot(t, map[string]string{"package.json": "{\"name\": \"a\", \"description\": \"no \\\"scripts\\\" here\"}\n"})
	assert.Empty(t, scriptFindings(slopfix.CheckTree(root).Findings))
}

// A dependency's manifest belongs to the dependency, so the walk never reads node_modules.
func TestNodeModulesIsNotRead(t *testing.T) {
	root := gitRoot(t, map[string]string{"node_modules/x/package.json": "{\"scripts\": {}}\n"})
	assert.Empty(t, scriptFindings(slopfix.CheckTree(root).Findings))
}

// A manifest this rule cannot parse is a check that did not happen, so it is a finding.
func TestAnUnparseablePackageJSONIsReported(t *testing.T) {
	root := gitRoot(t, map[string]string{"package.json": "{not json\n"})
	found := scriptFindings(slopfix.CheckTree(root).Findings)
	require.Len(t, found, 1)
	assert.Contains(t, found[0].Rule, "does not parse")
}

func TestOnlyPackageScriptsSelectsTheRule(t *testing.T) {
	root := gitRoot(t, map[string]string{"package.json": "{\"scripts\": {}}\n"})
	out := slopfix.CheckTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleRepo}, IDs: []string{slopfix.IDPackageScripts}})
	assert.Equal(t, []string{slopfix.IDPackageScripts}, ids(out.Findings))
}
