package bashclean

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// go-toolchain's whole output has to reach the transcript. Every spelling below
// moves it somewhere a grep reads instead, and the rewrite puts the run back on
// the terminal rather than blocking the command. The command that caused this
// rule was:
//
//	go-toolchain --generate <hash> > log; grep -nE "FAIL|Error:" log
func TestADivertedGoToolchainIsUndiverted(t *testing.T) {
	for in, want := range map[string]string{
		"go-toolchain > log":                "go-toolchain",
		"go-toolchain >> log":               "go-toolchain",
		"go-toolchain &> log":               "go-toolchain",
		"go-toolchain 1> log":               "go-toolchain",
		"go-toolchain > /dev/null":          "go-toolchain",
		"go-toolchain | head":               "go-toolchain",
		"go-toolchain | grep -n FAIL | wc":  "go-toolchain",
		"go-toolchain |& tee log":           "go-toolchain",
		"cd x && go-toolchain > log":        "cd x && go-toolchain",
		"sudo go-toolchain > log":           "sudo go-toolchain",
		"go-toolchain --generate abc > log": "go-toolchain --generate abc",
		"go-toolchain > log && git push":    "go-toolchain && git push",
		"go-toolchain | tee log; git push":  "go-toolchain\ngit push",
	} {
		got := Transform(in)
		assert.False(t, got.Denied, "wrongly denied: %s", in)
		assert.Equal(t, "set -o pipefail\n"+want+"\n", got.Command, "wrong rewrite: %s", in)
	}
}

// A redirect of stderr alone leaves the output on the terminal, so it survives
// while the stdout redirect beside it goes.
func TestStderrRedirectsSurviveTheRewrite(t *testing.T) {
	got := Transform("go-toolchain > log 2> errors")
	assert.Equal(t, "set -o pipefail\ngo-toolchain 2>errors\n", got.Command)
}

// A capture feeds a value to the rest of the script, so there is nothing to
// strip and the deny is the honest answer.
func TestACapturedGoToolchainIsDenied(t *testing.T) {
	for _, in := range []string{
		"out=$(go-toolchain)",
		"diff <(go-toolchain) old",
		"echo $(cd x && go-toolchain | head -1)",
	} {
		got := Transform(in)
		assert.True(t, got.Denied, "not denied: %s", in)
		assert.Equal(t, "toolchain_capture", got.Reason, "wrong reason: %s", in)
	}
}

// The controls. A bare run is the whole point of the rule, and stderr alone
// leaves the output on the terminal where the reader sees it.
func TestABareGoToolchainIsUntouched(t *testing.T) {
	for _, in := range []string{
		"go-toolchain",
		"go-toolchain --generate abc",
		"go-toolchain 2> errors",
		"go-toolchain && git push",
	} {
		got := Transform(in)
		assert.False(t, got.Denied, "wrongly denied: %s", in)
		assert.NotContains(t, got.Rules, "toolchain_output", "wrongly rewritten: %s", in)
	}
}

// The rule names a single command. Redirecting anything else is ordinary work,
// and the tee rule keeps that output visible its own way.
func TestRedirectingAnotherCommandIsAllowed(t *testing.T) {
	got := Transform("git status > log")
	assert.False(t, got.Denied)
	assert.Equal(t, "set -o pipefail\ngit status | tee log\n", got.Command)
}
