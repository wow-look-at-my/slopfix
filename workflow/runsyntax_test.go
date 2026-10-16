package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

func syntaxFindings(content string) []ste.Finding {
	var out []ste.Finding
	for _, f := range workflow.Check(content) {
		if f.ID == workflow.IDRunScriptSyntax {
			out = append(out, f)
		}
	}
	return out
}

// emptyIfBody is wow-look-at-my/sglang fork-ci.yml at c33f43989a. The job runs
// on the default branch alone, so no pull request ran the script, and master
// failed with: syntax error near unexpected token `fi'.
const emptyIfBody = "on:\n" +
	"  push:\n" +
	"    branches: ['**']\n" +
	"jobs:\n" +
	"  release:\n" +
	"    if: github.ref == 'refs/heads/master'\n" +
	"    runs-on: ubuntu-latest\n" +
	"    steps:\n" +
	"      - run: |\n" +
	"          shopt -s nullglob\n" +
	"          WHEELS=(dist/*.whl)\n" +
	"          if [ ${#WHEELS[@]} -eq 0 ]; then\n" +
	"          fi\n"

func TestAnIfWithNoBodyIsReportedOnItsThen(t *testing.T) {
	found := syntaxFindings(emptyIfBody)
	require.Len(t, found, 1)
	assert.Equal(t, 12, found[0].Line)
	assert.Contains(t, found[0].Detail, "then")
	assert.Contains(t, found[0].Detail, "statement list")
	assert.Contains(t, found[0].Fix, "bash -n")
	assert.False(t, found[0].Warning())
}

// The negative control: the same guard with its body parses.
func TestTheSameIfWithItsBodyPasses(t *testing.T) {
	content := emptyIfBody[:len(emptyIfBody)-len("          fi\n")] +
		"            echo \"::error::no sglang-kernel wheel was built\"; exit 1\n" +
		"          fi\n"
	assert.Empty(t, syntaxFindings(content))
}

func TestAScriptOnTheRunRowIsReportedThere(t *testing.T) {
	content := "jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo (\n"
	found := syntaxFindings(content)
	require.Len(t, found, 1)
	assert.Equal(t, 5, found[0].Line)
}

// A composite action's step names its shell, and only bash and sh are parsed.
func TestACompositeActionStepIsReadByItsShell(t *testing.T) {
	head := "runs:\n  using: composite\n  steps:\n    - shell: "
	body := "\n      run: |\n        if true; then\n        fi\n"
	for _, shell := range []string{"bash", "sh", "bash --noprofile --norc -eo pipefail {0}"} {
		assert.Len(t, syntaxFindings(head+shell+body), 1, shell)
	}
	for _, shell := range []string{"pwsh", "python", "cmd"} {
		assert.Empty(t, syntaxFindings(head+shell+body), shell)
	}
}

// sh is POSIX, where an array does not parse.
func TestAnShStepIsParsedAsPOSIX(t *testing.T) {
	content := "jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - shell: sh\n        run: |\n          X=(a b)\n"
	found := syntaxFindings(content)
	require.Len(t, found, 1)
	assert.Equal(t, 7, found[0].Line)
	assert.Empty(t, syntaxFindings("jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n          X=(a b)\n"))
}

// A step whose shell is pwsh, by the job's runner or a default, is not shell.
func TestAPowerShellStepIsNotParsed(t *testing.T) {
	broken := "        run: |\n          if ($x) { exit 1 }\n          fi\n"
	for _, content := range []string{
		"jobs:\n  a:\n    runs-on: windows-latest\n    steps:\n      - name: x\n" + broken,
		"defaults:\n  run:\n    shell: pwsh\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - name: x\n" + broken,
		"jobs:\n  a:\n    runs-on: ubuntu-latest\n    defaults:\n      run:\n        shell: pwsh\n    steps:\n      - name: x\n" + broken,
	} {
		assert.Empty(t, syntaxFindings(content), content)
	}
	bash := "jobs:\n  a:\n    runs-on: windows-latest\n    steps:\n      - shell: bash\n        run: |\n          fi\n"
	assert.Len(t, syntaxFindings(bash), 1)
}

// An expression is text the runner writes before the shell reads the script.
func TestAnExpressionParsesWhereAWordDoes(t *testing.T) {
	content := "jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n" +
		"          if [ \"${{ inputs.mode }}\" = fast ]; then\n" +
		"            echo ${{ github.sha }}\n" +
		"          fi\n" +
		"          N=$(( ${{ inputs.count }} + 1 ))\n" +
		"          ${{ inputs.command }} --flag '${{ matrix.os }}'\n"
	assert.Empty(t, syntaxFindings(content))
}

func repairedBy(content string) string {
	return workflow.Fix(content, func(id string) bool { return id == workflow.IDRunScriptSyntax }).Text
}

// An if with no body gets the no-op, and the rest of the file is as written.
func TestAnEmptyIfBodyGetsTheNoOp(t *testing.T) {
	fixed := repairedBy(emptyIfBody)
	assert.Contains(t, fixed, "then :\n")
	assert.Empty(t, syntaxFindings(fixed))
	assert.Contains(t, fixed, "WHEELS=(dist/*.whl)\n")
}

// A statement the script never closes gets its closer.
func TestAnUnclosedIfGetsItsFi(t *testing.T) {
	content := "jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n          if true; then\n            echo hi\n"
	fixed := repairedBy(content)
	assert.Empty(t, syntaxFindings(fixed))
	assert.Contains(t, fixed, "            echo hi\n          fi\n")
}

// A script on the run row is quoted whole once a line of it is a comment.
func TestAScriptOnTheRunRowIsRepaired(t *testing.T) {
	content := "jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo (\n"
	assert.Empty(t, syntaxFindings(repairedBy(content)))
}

// A script that parses is left as written.
func TestAParsingScriptIsNotRewritten(t *testing.T) {
	content := "jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n          echo hi\n"
	assert.Equal(t, content, repairedBy(content))
}
