package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
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

// fixing opens a workflow under the gates slopfix fix writes it through.
func fixing(content string) *fixer.File {
	return fixer.NewFile("", content, workflow.Options(fixer.Options{}))
}

// A guard with its body, for the deletion below.
const guardedBody = "on:\n" +
	"  push:\n" +
	"    branches: ['**']\n" +
	"jobs:\n" +
	"  release:\n" +
	"    runs-on: ubuntu-latest\n" +
	"    steps:\n" +
	"      - run: |\n" +
	"          if [ -z \"$WHEEL\" ]; then\n" +
	"            exit 1\n" +
	"          fi\n" +
	"      - uses: wow-look-at-my/slopfix@master\n" +
	"        continue-on-error: true\n"

// A deletion of the if's only body line leaves bash a then with nothing before
// fi. The gate refuses it, and the script keeps its text.
func TestFixKeepsAScriptADeletionWouldLeaveWithAnEmptyIfBody(t *testing.T) {
	f := fixing(guardedBody)
	res := f.Apply([]edit.Edit{edit.Rows(guardedBody, 9, 9, 0, nil)})
	assert.Equal(t, guardedBody, f.Text())
	assert.Empty(t, res.Applied)
	require.Len(t, res.Refused, 1)
	assert.Contains(t, res.Refused[0].Reason, "no longer parses as shell")
	assert.Empty(t, syntaxFindings(f.Text()))
}

// The negative control: a deletion outside the script lands in the same batch.
func TestADeletionOutsideTheScriptStillLands(t *testing.T) {
	f := fixing(guardedBody)
	res := f.Apply([]edit.Edit{edit.Rows(guardedBody, 9, 9, 0, nil), edit.Rows(guardedBody, 12, 12, 0, nil)})
	assert.Equal(t, guardedBody[:len(guardedBody)-len("        continue-on-error: true\n")], f.Text())
	assert.Len(t, res.Applied, 1)
	assert.Len(t, res.Refused, 1)
}

// A script that already fails does not hold every other repair back.
func TestAScriptThatFailedBeforeBlocksNoOtherEdit(t *testing.T) {
	f := fixing(emptyIfBody)
	res := f.Apply([]edit.Edit{edit.Rows(emptyIfBody, 5, 5, 0, nil)})
	assert.Len(t, res.Applied, 1)
	assert.NotContains(t, f.Text(), "if: github.ref")
	assert.Len(t, syntaxFindings(f.Text()), 1)
}
