package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunNamesItsOneArgument(t *testing.T) {
	assert.ErrorContains(t, run(nil), "usage: ratchet BRANCH_CHECKOUT")
	assert.ErrorContains(t, run([]string{"a", "b"}), "usage: ratchet BRANCH_CHECKOUT")
}

func TestRunFailsOnAHeadThatDoesNotBuild(t *testing.T) {
	t.Chdir("../..")
	assert.ErrorContains(t, run([]string{t.TempDir()}), "go build in")
}
