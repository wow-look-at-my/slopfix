package slopfix_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// fixEpoll runs the whole fix on a real Go source file from the cosmo syscall package.
func fixEpoll(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("testdata/epoll_cosmo.go.in")
	require.NoError(t, err)
	repair := slopfix.Fix(slopfix.Request{Content: string(src), Path: "src/syscall/epoll_cosmo.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	return repair.Text
}

func TestFixKeepsTheNosplitDirective(t *testing.T) {
	out := fixEpoll(t)
	assert.Contains(t, out, "\n//go:nosplit\nfunc darwinEpollSyscall(", "a space after the marker turns the directive off")
	assert.NotContains(t, out, "// go:nosplit")
}

// A yearly copyright bump pads commits, so the fix cuts the year.
func TestFixCutsTheCopyrightYear(t *testing.T) {
	out := fixEpoll(t)
	assert.True(t, strings.HasPrefix(out, "// Copyright The Go Authors."), out)
}

func TestFixKeepsANegation(t *testing.T) {
	prose := strings.Join(strings.Fields(strings.ReplaceAll(fixEpoll(t), "\n//", " ")), " ")
	assert.Contains(t, prose, "whose descriptor no longer refers to the file registered", "a cut of the negation says the opposite")
}

// The block cap still applies to the long comment under the imports.
func TestFixStillCapsTheTopComment(t *testing.T) {
	out := fixEpoll(t)
	block := regexp.MustCompile(`(?m)^// epoll on a macOS host\.(?:.*\n//.*)*`).FindString(out)
	require.NotEmpty(t, block)
	assert.LessOrEqual(t, strings.Count(block, "\n")+1, 12, block)
}
