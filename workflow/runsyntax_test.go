package workflow_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
	yaml "go.yaml.in/yaml/v3"
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

const stepsHead = "jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n"

// A script on the run row is written again as a quoted string, with the stray bracket quoted.
func TestAScriptOnTheRunRowIsRepaired(t *testing.T) {
	fixed := repairedBy(stepsHead + "      - run: echo (\n")
	assert.Equal(t, stepsHead+"      - run: \"echo '('\"\n", fixed)
	assert.Empty(t, syntaxFindings(fixed))
}

// A closing word with nothing to close is dropped, and every command stays.
func TestAStrayClosingWordIsDropped(t *testing.T) {
	fixed := repairedBy(stepsHead + "      - run: |\n          echo hi\n          fi\n")
	assert.Equal(t, stepsHead+"      - run: |\n          echo hi\n\n", fixed)
	assert.Empty(t, syntaxFindings(fixed))
}

// An sh step that uses a bash feature names bash, and its script stays as written.
func TestAnShStepWithAnArrayNamesBash(t *testing.T) {
	fixed := repairedBy(stepsHead + "      - shell: sh\n        run: |\n          X=(a b)\n")
	assert.Equal(t, stepsHead+"      - shell: bash\n        run: |\n          X=(a b)\n", fixed)
	assert.Empty(t, syntaxFindings(fixed))
}

// A line no rule mends passes to eval whole, so it still fails loudly at run time and no command is cut.
func TestAnotherRejectedLineGoesToEval(t *testing.T) {
	fixed := repairedBy(stepsHead + "      - run: |\n          echo hi\n          case\n")
	assert.Equal(t, stepsHead+"      - run: |\n          echo hi\n          eval 'case'\n", fixed)
	assert.Empty(t, syntaxFindings(fixed))
}

// Every input that reaches the eval fallback fails loudly when bash runs the repaired script.
func TestEveryEvalFallbackFailsLoudlyAtRunTime(t *testing.T) {
	for _, line := range []string{"case", "function"} {
		fixed := repairedBy(stepsHead + "      - run: |\n          " + line + "\n")
		assert.Contains(t, fixed, "eval '"+line+"'", line)
		var doc struct {
			Jobs map[string]struct {
				Steps []struct {
					Run string `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		require.NoError(t, yaml.Unmarshal([]byte(fixed), &doc))
		script := doc.Jobs["a"].Steps[0].Run
		out, err := exec.Command("bash", "-ec", script).CombinedOutput()
		require.Error(t, err, line)
		assert.Contains(t, string(out), "syntax error", line)
	}
}

// No repair comments a command out.
func TestNoRepairCommentsALineOut(t *testing.T) {
	for _, script := range []string{"echo (", "echo hi\nfi", "case", "echo )", "echo a &&"} {
		fixed := repairedBy(stepsHead + "      - run: |\n          " + strings.ReplaceAll(script, "\n", "\n          ") + "\n")
		assert.NotContains(t, fixed, "# ", script)
		assert.Empty(t, syntaxFindings(fixed), script)
	}
}

// A repeated key parses for its own repair alone: another repair of the file is refused until the key is dropped.
func TestARepeatedKeyStopsEveryRepairButItsOwn(t *testing.T) {
	content := stepsHead + "      - run: echo (\n        run: echo there\n"
	kept := func(id string) bool { return id == workflow.IDRunScriptSyntax }
	assert.Equal(t, content, workflow.Fix(content, kept).Text)
	dropped := workflow.Fix(content, func(id string) bool { return id == workflow.IDDuplicateStepKey })
	assert.NotEqual(t, content, dropped.Text)
	var parsed any
	require.NoError(t, yaml.Unmarshal([]byte(dropped.Text), &parsed))
}

// A script that parses is left as written.
func TestAParsingScriptIsNotRewritten(t *testing.T) {
	content := "jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n          echo hi\n"
	assert.Equal(t, content, repairedBy(content))
}
