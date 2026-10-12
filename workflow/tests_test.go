package workflow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/workflow"
)

func testFindings(t *testing.T, content string) []string {
	t.Helper()
	var out []string
	for _, finding := range workflow.Check(content) {
		if finding.ID == workflow.IDTestInYAML {
			out = append(out, finding.Detail)
		}
	}
	return out
}

func TestAnAssertionInARunBlockIsReported(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          grep -q ready out.txt || { echo '::error::missing'; exit 1; }\n"

	findings := testFindings(t, content)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0], "grep -q ready")
}

func TestABracketComparisonIsReported(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          [ \"$count\" = 3 ] || exit 1\n"

	assert.Len(t, testFindings(t, content), 1)
}

// A step that merely runs a command fails on its own exit code. That is not an
// assertion, and reporting it makes every workflow unwritable.
func TestAnOrdinaryCommandIsAllowed(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          npm ci\n          npm test\n          go-toolchain\n"

	assert.Empty(t, testFindings(t, content))
}

// An error annotation on its own is a report, not an expectation.
func TestAnErrorAnnotationAloneIsAllowed(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          echo '::error::the cache was cold'\n"

	assert.Empty(t, testFindings(t, content))
}

func TestAWorkflowThatWritesATestFileIsReported(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          echo 'package main' > cache_test.go\n"

	findings := testFindings(t, content)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0], "cache_test.go")
}

func TestWritingAnOrdinaryFileIsAllowed(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          echo hello > out.txt\n"

	assert.Empty(t, testFindings(t, content))
}

func TestAnAssertionHelperIsReported(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          assert_equal() {\n            [ \"$1\" = \"$2\" ]\n          }\n"

	findings := testFindings(t, content)
	require.NotEmpty(t, findings)
	assert.Contains(t, findings[0], "assert_equal")
}

// A run: block ends at anything indented no further than its own key.
func TestABlockEndsAtTheNextKey(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          npm ci\n        env:\n          GREP: grep -q x || exit 1\n"

	assert.Empty(t, testFindings(t, content))
}

func TestAOneLineRunIsRead(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: test -f out.txt || exit 1\n"

	assert.Len(t, testFindings(t, content), 1)
}

func TestALongLineIsTruncatedInTheEvidence(t *testing.T) {
	long := "grep -q ready " + strings.Repeat("x", 200) + " || exit 1"
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          " + long + "\n"

	findings := testFindings(t, content)
	require.Len(t, findings, 1)
	assert.Len(t, findings[0], workflow.EvidenceCap)
	assert.True(t, strings.HasSuffix(findings[0], "..."))
}

// A step that runs PowerShell holds no shell test, so the rule does not read it.
func TestAPowerShellStepIsNotRead(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    runs-on: windows-latest\n    steps:\n      - run: test -f out.txt || exit 1\n      - shell: pwsh\n        run: test -f out.txt || exit 1\n"

	assert.Empty(t, testFindings(t, content))
}

const extractHead = "on: {push: {branches: ['**']}}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n"

// extracted runs the repair on a workflow under .github/workflows of a fresh
// directory. It answers the workflow and each file it created, by the path
// from that directory.
func extracted(t *testing.T, content string) (string, map[string]string) {
	t.Helper()
	root := t.TempDir()
	text, created := workflow.Extract(filepath.Join(root, ".github", "workflows", "ci.yml"), content)
	files := map[string]string{}
	for _, c := range created {
		rel, err := filepath.Rel(root, c.Path)
		require.NoError(t, err)
		files[filepath.ToSlash(rel)] = c.Text
	}
	return text, files
}

// The whole script moves into a file, every line of it, and the step runs the
// file. A guard keeps its body, so bash never meets an if with an empty then.
func TestTheRepairMovesTheWholeScript(t *testing.T) {
	content := extractHead + "      - run: |\n          npm ci\n          if [ ! -f out.txt ]; then\n            echo \"::error::no out.txt\"; exit 1\n          fi\n      - run: npm test\n"

	text, files := extracted(t, content)
	assert.Equal(t, extractHead+"      - run: bash .github/scripts/ci-x-1.sh\n      - run: npm test\n", text)
	assert.Equal(t, map[string]string{".github/scripts/ci-x-1.sh": "#!/usr/bin/env bash\nset -eo pipefail\n" +
		"npm ci\nif [ ! -f out.txt ]; then\n  echo \"::error::no out.txt\"; exit 1\nfi\n"}, files)
	assert.Empty(t, testFindings(t, text))
}

// Each distinct expression becomes an argument, and the script reads the
// argument where the expression stood, quoted the way the place needs.
func TestAnExpressionBecomesAnArgument(t *testing.T) {
	content := extractHead + "      - run: |\n" +
		"          grep -q ${{ github.sha }} out.txt || exit 1\n" +
		"          echo \"built ${{ github.sha }} on ${{ runner.os }}\"\n" +
		"          echo '${{ runner.os }}'\n"

	text, files := extracted(t, content)
	assert.Equal(t, extractHead+"      - run: bash .github/scripts/ci-x-1.sh \"${{ github.sha }}\" \"${{ runner.os }}\"\n", text)
	assert.Equal(t, "#!/usr/bin/env bash\nset -eo pipefail\n"+
		"grep -q \"$1\" out.txt || exit 1\n"+
		"echo \"built $1 on $2\"\n"+
		"echo ''\"$2\"''\n", files[".github/scripts/ci-x-1.sh"])
	assert.Empty(t, testFindings(t, text))
	for _, f := range workflow.Check(text) {
		assert.NotEqual(t, workflow.IDEnvIndirection, f.ID, f.String())
	}
}

// A function reads its own arguments, so a script whose function holds an
// expression keeps each value in a variable and clears the arguments first.
func TestAnExpressionInAFunctionIsHeldInAVariable(t *testing.T) {
	content := extractHead + "      - run: |\n" +
		"          assert_sha() { grep -q \"${{ github.sha }}\" \"$1\" || exit 1; }\n" +
		"          assert_sha out.txt\n"

	_, files := extracted(t, content)
	assert.Equal(t, "#!/usr/bin/env bash\nset -eo pipefail\n"+
		"slopfix_arg_1=\"$1\"\nset --\n"+
		"assert_sha() { grep -q \"${slopfix_arg_1}\" \"$1\" || exit 1; }\n"+
		"assert_sha out.txt\n", files[".github/scripts/ci-x-1.sh"])
}

// A step that names sh runs the file with sh, under sh's own errexit.
func TestAShStepRunsTheFileWithSh(t *testing.T) {
	content := extractHead + "      - shell: sh\n        run: test -f out.txt || exit 1\n"

	text, files := extracted(t, content)
	assert.Contains(t, text, "        run: sh .github/scripts/ci-x-1.sh\n")
	assert.Equal(t, "#!/bin/sh\nset -e\ntest -f out.txt || exit 1\n", files[".github/scripts/ci-x-1.sh"])
}

func TestAShellTemplateRunsTheFile(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\ndefaults:\n  run:\n    shell: bash -x {0}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: test -f out.txt || exit 1\n"

	text, files := extracted(t, content)
	assert.Contains(t, text, "      - run: bash -x .github/scripts/ci-x-1.sh\n")
	assert.Equal(t, "#!/usr/bin/env bash\ntest -f out.txt || exit 1\n", files[".github/scripts/ci-x-1.sh"])
}

// A step that starts in another directory reaches the file from the workspace.
func TestAWorkingDirectoryReachesTheFileFromTheWorkspace(t *testing.T) {
	content := extractHead + "      - working-directory: sub\n        run: test -f out.txt || exit 1  # the build output\n"

	text, _ := extracted(t, content)
	assert.Contains(t, text, "        run: bash \"$GITHUB_WORKSPACE/.github/scripts/ci-x-1.sh\"  # the build output\n")
}

// A file of that name holding other text stays, and the script takes the next name.
func TestAHeldNameTakesTheNextOne(t *testing.T) {
	root := t.TempDir()
	scripts := filepath.Join(root, ".github", "scripts")
	require.NoError(t, os.MkdirAll(scripts, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(scripts, "ci-x-1.sh"), []byte("echo other\n"), 0o644))
	content := extractHead + "      - run: test -f out.txt || exit 1\n"

	text, created := workflow.Extract(filepath.Join(root, ".github", "workflows", "ci.yml"), content)
	assert.Contains(t, text, "run: bash .github/scripts/ci-x-1-2.sh\n")
	require.Len(t, created, 1)
	assert.Equal(t, filepath.Join(scripts, "ci-x-1-2.sh"), created[0].Path)
}

// A driver that writes the workflow alone gets no change.
func TestTheRepairNeedsADriverThatWritesTheScript(t *testing.T) {
	content := extractHead + "      - run: test -f out.txt || exit 1\n"

	repair := workflow.Fix(content, func(id string) bool { return id == workflow.IDTestInYAML })
	assert.False(t, repair.Changed)
	assert.Equal(t, content, repair.Text)
}

// A workflow that holds no test comes back unchanged.
func TestTheRepairLeavesACleanWorkflowAlone(t *testing.T) {
	content := extractHead + "      - run: |\n          npm ci\n"

	text, files := extracted(t, content)
	assert.Equal(t, content, text)
	assert.Empty(t, files)
}
