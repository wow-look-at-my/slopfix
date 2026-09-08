package bashclean

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every rule here resolves the program through shellwalk, so one entry for
// `rm` covers the wrapper prefixes and the absolute path too. The hand-rolled
// resolver this replaced saw `command`, `builtin` and a leading backslash
// only, so `sudo rm -rf build` reached the shell untouched.
func TestWrappedCommandsResolve(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"sudo rm -rf build", "set -o pipefail\nsudo recycler trash build\n"},
		{"sudo -u root rm -rf build", "set -o pipefail\nsudo -u root recycler trash build\n"},
		{"env rm -f a", "set -o pipefail\nenv recycler trash a\n"},
		{"env FOO=1 rm -f a", "set -o pipefail\nenv FOO=1 recycler trash a\n"},
		{"nice -n 10 rm a", "set -o pipefail\nnice -n 10 recycler trash a\n"},
		{"timeout 5 rm a", "set -o pipefail\ntimeout 5 recycler trash a\n"},
		{"/bin/rm -rf build", "set -o pipefail\nrecycler trash build\n"},
		{"nohup rm a", "set -o pipefail\nnohup recycler trash a\n"},
		{"busybox rm a", "set -o pipefail\nbusybox recycler trash a\n"},
		{"xargs rm", "set -o pipefail\nxargs recycler trash\n"},
	} {
		got := Transform(tc.in)
		assert.False(t, got.Denied, "%s: unexpected deny (%s)", tc.in, got.Reason)
		assert.Equal(t, tc.want, got.Command, tc.in)
	}
}

// A wrapped invocation reaches the denies too.
func TestWrappedCommandsDeny(t *testing.T) {
	for _, in := range []string{
		"sudo head -60 notes.md",
		"env cat secret.txt",
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
