package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const envAction = "name: x\n" +
	"runs:\n" +
	"  using: composite\n" +
	"  steps:\n" +
	"    - id: download\n" +
	"      uses: some/download@v1\n" +
	"    - name: install\n" +
	"      shell: bash\n" +
	"      env:\n" +
	"        BIN_DIR: ${{ runner.temp }}/go-toolchain-bin\n" +
	"        DOWNLOADED: ${{ steps.download.outputs.path }}\n" +
	"      run: |\n" +
	"        set -euo pipefail\n" +
	"        BIN_DIR=\"$RUNNER_TEMP/go-toolchain-bin\"\n" +
	"        mkdir -p \"${BIN_DIR}\"\n" +
	"        mv \"${DOWNLOADED}\" \"${BIN_DIR}/go-toolchain\"\n"

const envActionFixed = "name: x\n" +
	"runs:\n" +
	"  using: composite\n" +
	"  steps:\n" +
	"    - id: download\n" +
	"      uses: some/download@v1\n" +
	"    - name: install\n" +
	"      shell: bash\n" +
	"      env:\n" +
	"        DOWNLOADED: ${{ steps.download.outputs.path }}\n" +
	"      run: |\n" +
	"        set -euo pipefail\n" +
	"        BIN_DIR=\"$RUNNER_TEMP/go-toolchain-bin\"\n" +
	"        mkdir -p \"${BIN_DIR}\"\n" +
	"        mv \"${DOWNLOADED}\" \"${BIN_DIR}/go-toolchain\"\n"

// The script sets BIN_DIR before it reads it, so the entry goes. A step output
// is text another program wrote, so DOWNLOADED stays in env.
func TestAnEnvEntryTheScriptOverwritesGoes(t *testing.T) {
	found := envIndirections(envAction)
	require.Len(t, found, 1)
	assert.Equal(t, 10, found[0].Line)
	assert.Equal(t, IDEnvIndirection, found[0].ID)

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
	return "on:\n" +
		"  workflow_dispatch:\n" +
		"    inputs:\n" +
		"      image_tag:\n" +
		"        type: string\n" +
		"      untyped:\n" +
		"        description: no type\n" +
		"      build_docker:\n" +
		"        type: boolean\n" +
		"      flavor:\n" +
		"        type: choice\n" +
		"        options: [a, b]\n" +
		"  workflow_call:\n" +
		"    inputs:\n" +
		"      called:\n" +
		"        type: string\n" +
		"      count:\n" +
		"        type: number\n" +
		"jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - env:\n" +
		"          VALUE: " + entry + "\n" +
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
	content := "name: x\n" +
		"inputs:\n" +
		"  flag:\n" +
		"    default: 'false'\n" +
		"runs:\n" +
		"  using: composite\n" +
		"  steps:\n" +
		"    - shell: bash\n" +
		"      env:\n" +
		"        FLAG: ${{ inputs.flag }}\n" +
		"      run: echo \"$FLAG\"\n"
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

// A step output or a matrix value can hold quotes, as secret-server's JSON
// secrets output does. Inlined, its quotes end the script's quoted word early,
// so the entry stays in env, unreported.
func TestAStepOutputOrAMatrixValueStaysInEnv(t *testing.T) {
	for _, expr := range []string{"${{ steps.fetch.outputs.secrets }}", "${{ steps.x.outputs.result }}", "${{ matrix.os }}", "${{ github.ref_name }}", "${{ env.OTHER }}"} {
		content := "jobs:\n" +
			"  a:\n" +
			"    runs-on: ubuntu-latest\n" +
			"    steps:\n" +
			"      - id: fetch\n" +
			"        uses: some/action@v1\n" +
			"      - env:\n" +
			"          SECRETS_JSON: " + expr + "\n" +
			"        run: |\n" +
			"          if [ -z \"$SECRETS_JSON\" ]; then\n" +
			"            exit 1\n" +
			"          fi\n" +
			"          echo \"$SECRETS_JSON\" | jq empty\n"
		assert.Empty(t, envIndirections(content), expr)
		assert.Equal(t, content, Fix(content, func(string) bool { return true }).Text, expr)
	}
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
		"          OUT: ${{ github.sha }}\n" +
		"        run: git show \"$OUT\"\n"
	want := "jobs:\n" +
		"  a:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - env:\n" +
		"          KEEP: ${{ inputs.keep }}\n" +
		"        run: git show \"${{ github.sha }}\"\n"
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
	content := "name: x\n" +
		"runs:\n" +
		"  using: composite\n" +
		"  steps:\n" +
		"    - shell: bash\n" +
		"      run: ls \"$RUNNER_TEMP\"\n"
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
