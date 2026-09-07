package workflow_test

import (
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
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: |\n          grep -q ready out.txt || { echo '::error::missing'; exit 1; }\n"

	findings := testFindings(t, content)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0], "grep -q ready")
}

func TestABracketComparisonIsReported(t *testing.T) {
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: |\n          [ \"$count\" = 3 ] || exit 1\n"

	assert.Len(t, testFindings(t, content), 1)
}

// A step that merely runs a command fails on its own exit code. That is not an
// assertion, and reporting it makes every workflow unwritable.
func TestAnOrdinaryCommandIsAllowed(t *testing.T) {
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: |\n          npm ci\n          npm test\n          go-toolchain\n"

	assert.Empty(t, testFindings(t, content))
}

// An error annotation on its own is a report, not an expectation.
func TestAnErrorAnnotationAloneIsAllowed(t *testing.T) {
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: |\n          echo '::error::the cache was cold'\n"

	assert.Empty(t, testFindings(t, content))
}

func TestAWorkflowThatWritesATestFileIsReported(t *testing.T) {
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: |\n          echo 'package main' > cache_test.go\n"

	findings := testFindings(t, content)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0], "cache_test.go")
}

func TestWritingAnOrdinaryFileIsAllowed(t *testing.T) {
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: |\n          echo hello > out.txt\n"

	assert.Empty(t, testFindings(t, content))
}

func TestAnAssertionHelperIsReported(t *testing.T) {
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: |\n          assert_equal() {\n            [ \"$1\" = \"$2\" ]\n          }\n"

	findings := testFindings(t, content)
	require.NotEmpty(t, findings)
	assert.Contains(t, findings[0], "assert_equal")
}

// A run: block ends at anything indented no further than its own key.
func TestABlockEndsAtTheNextKey(t *testing.T) {
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: |\n          npm ci\n        env:\n          GREP: grep -q x || exit 1\n"

	assert.Empty(t, testFindings(t, content))
}

func TestAOneLineRunIsRead(t *testing.T) {
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: test -f out.txt || exit 1\n"

	assert.Len(t, testFindings(t, content), 1)
}

func TestALongLineIsTruncatedInTheEvidence(t *testing.T) {
	long := "grep -q ready " + strings.Repeat("x", 200) + " || exit 1"
	content := "on: push\njobs:\n  x:\n    steps:\n      - run: |\n          " + long + "\n"

	findings := testFindings(t, content)
	require.Len(t, findings, 1)
	assert.Len(t, findings[0], workflow.EvidenceCap)
	assert.True(t, strings.HasSuffix(findings[0], "..."))
}
