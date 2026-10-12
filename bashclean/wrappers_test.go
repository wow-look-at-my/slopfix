package bashclean

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every rule resolves the program through shellwalk, so the `rm` entry covers
// the wrapper prefixes and the absolute path too. The hand-rolled resolver this
// replaced saw `command`, `builtin` and a leading backslash, so `sudo rm -rf
// build` reached the shell untouched.
func TestWrappedCommandsResolve(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"sudo rm -rf build", "sudo recycler trash build\n"},
		{"sudo -u root rm -rf build", "sudo -u root recycler trash build\n"},
		{"env rm -f a", "env recycler trash a\n"},
		{"env FOO=1 rm -f a", "env FOO=1 recycler trash a\n"},
		{"nice -n 10 rm a", "nice -n 10 recycler trash a\n"},
		{"timeout 5 rm a", "timeout 5 recycler trash a\n"},
		{"/bin/rm -rf build", "recycler trash build\n"},
		{"nohup rm a", "nohup recycler trash a\n"},
		{"busybox rm a", "busybox recycler trash a\n"},
		{"xargs rm", "xargs recycler trash\n"},
	} {
		got := Transform(tc.in)
		assert.False(t, got.Denied, "%s: unexpected deny (%s)", tc.in, got.Reason)
		assert.Equal(t, tc.want, got.Command, tc.in)
	}
}

// A wrapped invocation reaches the denies too.
func TestWrappedCommandsDeny(t *testing.T) {
	for _, in := range []string{
		"sudo shred secret.txt",
		"timeout 5 rm --one-file-system x",
		"/usr/bin/perl -e 'print 1'",
		"sudo truncate -s 0 -r ref target",
	} {
		assert.True(t, Transform(in).Denied, in)
	}
}

// A lookup prints a name rather than running it, and a non-static command word
// is not knowable, so neither resolves to a program.
func TestLookupAndOpaqueCommandsResolveToNothing(t *testing.T) {
	for _, in := range []string{"command -v rm", "command -V rm", "$TOOL rm -rf build"} {
		got := Transform(in)
		assert.False(t, got.Denied, in)
		assert.NotContains(t, got.Command, "recycler", in)
	}
}
