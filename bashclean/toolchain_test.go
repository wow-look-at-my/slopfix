package bashclean

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// go-toolchain refuses a pipe so its whole output reaches the transcript. Every
// spelling below moves that output somewhere a grep reads instead, which is the
// same act. The command that caused this rule was:
//
//	go-toolchain --generate <hash> > log; grep -nE "FAIL|Error:" log
func TestADivertedGoToolchainIsDenied(t *testing.T) {
	for _, in := range []string{
		"go-toolchain > log",
		"go-toolchain >> log",
		"go-toolchain &> log",
		"go-toolchain > log 2>&1",
		"go-toolchain 1> log",
		"go-toolchain > /dev/null",
		"go-toolchain | head",
		"go-toolchain |& tee log",
		"out=$(go-toolchain)",
		"diff <(go-toolchain) old",
		"cd x && go-toolchain > log",
		"sudo go-toolchain > log",
		"go-toolchain --generate abc > log 2>&1",
	} {
		got := Transform(in)
		assert.True(t, got.Denied, "not denied: %s", in)
		assert.Equal(t, "toolchain_output", got.Reason, "wrong reason: %s", in)
	}
}

// The controls. A bare run is the whole point of the rule, and stderr alone
// leaves the output on the terminal where the reader sees it.
func TestABareGoToolchainIsAllowed(t *testing.T) {
	for _, in := range []string{
		"go-toolchain",
		"go-toolchain --generate abc",
		"go-toolchain 2> errors",
		"go-toolchain && git push",
	} {
		got := Transform(in)
		assert.False(t, got.Denied, "wrongly denied: %s", in)
	}
}

// The rule names a single command. Redirecting anything else is ordinary work.
func TestRedirectingAnotherCommandIsAllowed(t *testing.T) {
	got := Transform("git status > log")
	assert.False(t, got.Denied)
}
