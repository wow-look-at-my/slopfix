package workflow

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/env-action.yml
var envAction string

//go:embed testdata/env-action-fixed.yml
var envActionFixed string

func TestAnEnvEntryThatOnlyCarriesAnExpressionIsInlined(t *testing.T) {
	found := envIndirections(envAction)
	require.Len(t, found, 2)
	lines := []int{found[0].Line, found[1].Line}
	assert.ElementsMatch(t, []int{10, 11}, lines)
	for _, f := range found {
		assert.Equal(t, IDEnvIndirection, f.ID)
	}

	got := Fix(envAction, func(id string) bool { return id == IDEnvIndirection })
	assert.Equal(t, envActionFixed, got.Text)
	assert.Empty(t, envIndirections(got.Text))
}

func TestTheRuleRunsAsPartOfAFix(t *testing.T) {
	got := Fix(envAction, func(string) bool { return true })
	assert.Equal(t, envActionFixed, got.Text)
}

func TestAnEntryTheScriptNeverExpandsStays(t *testing.T) {
	content := "jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - run: go build ./...\n" +
		"        env:\n" +
		"          GOFLAGS: ${{ inputs.flags }}\n"
	assert.Empty(t, envIndirections(content))
}

// The env indirection is the defense against script injection. It stays.
func TestAnUntrustedExpressionStaysInEnv(t *testing.T) {
	content := "jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - env:\n" +
		"          TITLE: ${{ github.event.pull_request.title }}\n" +
		"        run: echo \"$TITLE\"\n"
	assert.Empty(t, envIndirections(content))
}

// dispatched is a workflow whose dispatch takes a free-text tag and a boolean,
// with one step reading the env entry the caller names.
func dispatched(entry string) string {
	return dispatchedYml + entry + "\n" +
		"        run: |\n" +
		"          if [ -n \"$VALUE\" ]; then\n" +
		"            IMAGE_TAGS=\"${VALUE}\"\n" +
		"          fi\n"
}

// An expression an outside party writes is shell injection once it is
// expanded inside run:, so the entry stays where it is, unreported.
func TestAnExpressionAnOutsiderWritesStaysInEnv(t *testing.T) {
	for _, expr := range []string{
		"${{ inputs.image_tag }}",
		"${{ inputs.untyped }}",
		"${{ inputs.flavor }}",
		"${{ inputs.called }}",
		"${{ inputs['build_docker'] }}",
		"${{ inputs.undeclared }}",
		"${{ inputs.build_docker && inputs.image_tag }}",
		"${{ github.event.inputs.image_tag }}",
		"${{ github.event.head_commit.message }}",
		"${{ github.event.pull_request.body }}",
		"${{ github.head_ref }}",
	} {
		content := dispatched(expr)
		assert.Empty(t, envIndirections(content), expr)
		got := Fix(content, func(string) bool { return true })
		assert.Equal(t, content, got.Text, expr)
	}
}

// An expression whose value cannot carry shell text still goes inline.
func TestAnExpressionWithNoOutsideTextIsInlined(t *testing.T) {
	for _, expr := range []string{
		"${{ inputs.build_docker }}",
		"${{ inputs.count }}",
		"${{ github.event_name }}",
		"${{ runner.temp }}",
	} {
		content := dispatched(expr)
		require.Len(t, envIndirections(content), 1, expr)
		got := Fix(content, func(id string) bool { return id == IDEnvIndirection })
		assert.Contains(t, got.Text, "IMAGE_TAGS=\""+expr+"\"", expr)
		assert.NotContains(t, got.Text, "VALUE:", expr)
	}
}

// Every input of a composite action is text its caller writes.
func TestACompositeActionInputStaysInEnv(t *testing.T) {
	content := testACompositeActionInputStaysInEnvYml
	assert.Empty(t, envIndirections(content))
}

// A script action's result is JSON, and its quotes end a quoted script word
// early. A secret written into the script lands in the script file on disk.
func TestAScriptResultOrASecretStaysInEnv(t *testing.T) {
	for _, uses := range []string{"./typescript", "wow-look-at-my/actions@typescript#latest", "actions/github-script@v7"} {
		content := "jobs:\n" +
			"  a:\n" +
			"    runs-on: ubuntu-latest\n" +
			"    steps:\n" +
			"      - id: ts\n" +
			"        uses: " + uses + "\n" +
			"      - env:\n" +
			"          VALUE: ${{ steps.ts.outputs.result }}\n" +
			"          TOKEN: ${{ secrets.TOKEN }}\n" +
			"        run: echo \"$VALUE $TOKEN\"\n"
		assert.Empty(t, envIndirections(content), uses)
	}
}

// Another action's result output is no JSON the rule knows of, so it goes inline.
func TestAnotherActionsResultIsInlined(t *testing.T) {
	content := "jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - id: x\n" +
		"        uses: some/action@v1\n" +
		"      - env:\n" +
		"          VALUE: ${{ steps.x.outputs.result }}\n" +
		"        run: echo \"$VALUE\"\n"
	assert.Len(t, envIndirections(content), 1)
}

func TestAnEntryWithExtraTextOrAnOperatorStays(t *testing.T) {
	content := "jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - env:\n" +
		"          A: prefix-${{ inputs.a }}\n" +
		"          B: ${{ inputs.b }}\n" +
		"        run: |\n" +
		"          echo \"$A\" \"${B:-none}\"\n"
	assert.Empty(t, envIndirections(content))
}

func TestASingleLineRunIsInlinedAndKeepsTheOtherEntries(t *testing.T) {
	content := "jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - env:\n" +
		"          KEEP: ${{ inputs.keep }}\n" +
		"          OUT: ${{ steps.x.outputs.path }}\n" +
		"        run: cat \"$OUT\"\n"
	want := "jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - env:\n" +
		"          KEEP: ${{ inputs.keep }}\n" +
		"        run: cat \"${{ steps.x.outputs.path }}\"\n"
	got := Fix(content, func(id string) bool { return id == IDEnvIndirection })
	assert.Equal(t, want, got.Text)
}

// Inside a container job the runner's paths differ from the container's.
func TestAContainerJobKeepsTheRunnerVariable(t *testing.T) {
	content := "jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    container: alpine\n" +
		"    steps:\n" +
		"      - run: ls \"$RUNNER_TEMP\"\n"
	assert.Empty(t, envIndirections(content))
}

// A composite action can run in a container job, where the expression names the
// host path and the variable names the mount.
func TestACompositeActionKeepsTheRunnerVariable(t *testing.T) {
	content := testACompositeActionKeepsTheRunnerVariableYml
	assert.Empty(t, envIndirections(content))
}

func TestAPlainJobStillNamesTheRunnerContext(t *testing.T) {
	content := "jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - run: ls \"$RUNNER_TEMP\"\n"
	assert.Len(t, envIndirections(content), 1)
}

func TestAWindowsJobWithNoShellIsNotRead(t *testing.T) {
	content := "jobs:\n" +
		"  a:\n" +
		"    runs-on: windows-latest\n" +
		"    steps:\n" +
		"      - env:\n" +
		"          OUT: ${{ steps.x.outputs.path }}\n" +
		"        run: echo $OUT\n"
	assert.Empty(t, envIndirections(content))
}

//go:embed testdata/dispatched.yml
var dispatchedYml string

//go:embed testdata/test-a-composite-action-input-stays-in-env.yml
var testACompositeActionInputStaysInEnvYml string

//go:embed testdata/test-a-composite-action-keeps-the-runner-variable.yml
var testACompositeActionKeepsTheRunnerVariableYml string
