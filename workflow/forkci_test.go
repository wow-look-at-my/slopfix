package workflow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/workflow"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(body)
}

func everyRule(string) bool { return true }

// The guards annotate and exit inside an if body. Deleting the line leaves
// `then` with nothing before `fi`, which bash refuses to parse.
func TestARepairKeepsEveryLineOfARunScript(t *testing.T) {
	content := fixture(t, "fork-ci-guards.yml.in")

	repair := workflow.Fix(content, everyRule)

	assert.Contains(t, repair.Text, "if [ ${#WHEELS[@]} -eq 0 ]; then\n"+
		"            echo \"::error::no sglang-kernel wheel was built\"; exit 1\n"+
		"          fi\n")
	assert.Contains(t, repair.Text, "if [ ${#SOURCES[@]} -eq 0 ]; then\n"+
		"              echo \"::error::no per-arch image was pushed for ${TAG}\"; exit 1\n"+
		"            fi\n")
}

// The rule still reports what it reads as a test. The author answers it,
// because what a repair would cut is code.
func TestATestInAWorkflowIsReportedAndLeftInPlace(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          grep -q ready out.txt || { echo '::error::missing'; exit 1; }\n"
	require.NotEmpty(t, testFindings(t, content))

	repair := workflow.Fix(content, func(id string) bool { return id == workflow.IDTestInYAML })

	assert.False(t, repair.Changed)
	assert.Empty(t, repair.Removed)
}

// The gate refuses any edit that takes a row out of a script, whichever fixer
// writes it.
func TestTheGateRefusesAnEditThatDeletesAScriptRow(t *testing.T) {
	content := fixture(t, "fork-ci-guards.yml.in")
	row := rowOf(t, content, `echo "::error::no sglang-kernel wheel was built"`)
	f := fixer.NewFile("fork-ci.yml", content, workflow.Options(fixer.Options{}))

	result := f.Apply([]edit.Edit{edit.Rows(content, row, row, 0, nil)})

	assert.Equal(t, content, f.Text())
	assert.NotEmpty(t, result.Refused)
}

// A rewrite that keeps the row count but breaks the shell is refused too.
func TestTheGateRefusesAnEditThatBreaksTheShell(t *testing.T) {
	content := fixture(t, "fork-ci-guards.yml.in")
	row := rowOf(t, content, "          fi")
	f := fixer.NewFile("fork-ci.yml", content, workflow.Options(fixer.Options{}))

	result := f.Apply([]edit.Edit{edit.Rows(content, row, row, 0, []string{"          true"})})

	assert.Equal(t, content, f.Text())
	assert.NotEmpty(t, result.Refused)
}

// A rewrite that keeps the rows and still parses goes through, expressions
// included, so the env repair keeps working.
func TestTheGateAdmitsARewriteThatKeepsTheScript(t *testing.T) {
	content := fixture(t, "fork-ci-guards.yml.in")
	row := rowOf(t, content, `          TAG="sglang-kernel-v${VERSION}"`)
	f := fixer.NewFile("fork-ci.yml", content, workflow.Options(fixer.Options{}))

	f.Apply([]edit.Edit{edit.Rows(content, row, row, 0, []string{`          TAG="sglang-kernel-v${{ needs.fork-plan.outputs.kernel_version }}"`})})

	assert.Contains(t, f.Text(), `TAG="sglang-kernel-v${{ needs.fork-plan.outputs.kernel_version }}"`)
}

// An if body that annotates and exits is a guard on the step's own work,
// not a test.
func TestAnIfGuardThatAnnotatesAndExitsIsNoTest(t *testing.T) {
	assert.Empty(t, testFindings(t, fixture(t, "fork-ci-guards.yml.in")))
}

// A case arm annotates and exits with no comparison word. Its pattern is the
// comparison, so it is still a test.
func TestACaseArmThatAnnotatesAndExitsIsATest(t *testing.T) {
	for _, arm := range []string{
		`*) echo "::error::unexpected $STATE"; exit 1 ;;`,
		`failed|cancelled) echo "::error::run $STATE"; exit 2 ;;`,
		`(skipped) echo '::error::skipped'; exit 1 ;;`,
	} {
		content := "on: {push: {branches: ['**']}}\njobs:\n  x:\n    steps:\n      - run: |\n          case \"$STATE\" in\n            ok) ;;\n            " + arm + "\n          esac\n"
		assert.Len(t, testFindings(t, content), 1, arm)
	}
}

func rowOf(t *testing.T, content, prefix string) int {
	t.Helper()
	for i, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), strings.TrimSpace(prefix)) {
			return i
		}
	}
	t.Fatalf("no row starts with %q", prefix)
	return -1
}

// A header of prose and column-aligned rows is a comment block like any
// other: every row counts toward the limit, and the run is a single finding.
func TestACommentTableIsACommentBlock(t *testing.T) {
	content := fixture(t, "fork-ci-header.yml.in")

	findings := workflow.Check(content)

	require.Len(t, findings, 1)
	assert.Equal(t, workflow.IDCommentBlock, findings[0].ID)
	assert.Equal(t, "lines 3-17", findings[0].Detail)
	assert.Empty(t, workflow.Check(workflow.Fix(content, everyRule).Text))
}
