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
	"      run: |\n" +
	"        set -euo pipefail\n" +
	"        BIN_DIR=\"${{ runner.temp }}/go-toolchain-bin\"\n" +
	"        mkdir -p \"${BIN_DIR}\"\n" +
	"        mv \"${{ steps.download.outputs.path }}\" \"${BIN_DIR}/go-toolchain\"\n"

func TestAnEnvEntryThatOnlyCarriesAnExpressionIsInlined(t *testing.T) {
	found := envIndirections(envAction)
	require.Len(t, found, 3)
	lines := []int{found[0].Line, found[1].Line, found[2].Line}
	assert.ElementsMatch(t, []int{10, 11, 14}, lines)
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

// A JSON result holds quotes, so written into "..." it ends the string early.
// A secret written into the script lands in the script file on disk.
func TestARunTimeOutputOrASecretStaysInEnv(t *testing.T) {
	for _, expr := range []string{"steps.ts.outputs.result", "needs.build.outputs.version", "secrets.TOKEN"} {
		content := "jobs:\n" +
			"  a:\n" +
			"    runs-on: ubuntu-latest\n" +
			"    steps:\n" +
			"      - env:\n" +
			"          VALUE: ${{ " + expr + " }}\n" +
			"        run: echo \"$VALUE\"\n"
		assert.Empty(t, envIndirections(content), expr)
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
