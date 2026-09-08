package bashclean

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTransformCoreRules(t *testing.T) {
	cases := []struct{ in, want string }{
		{"rm -rf build", "set -o pipefail\nrecycler trash build\n"},
		{"docker compose restart api", "set -o pipefail\ndocker compose up -d --force-recreate api\n"},
		{"gh run view 123 --log-failed", "set -o pipefail\ngh wait-ci log 123 --failed\n"},
		{"sleep 30", "set -o pipefail\nsleep 3\n"},
	}
	for _, tc := range cases {
		got := Transform(tc.in)
		assert.False(t, got.Denied || got.Command != tc.want)

	}
}

func TestTransformDeniesHeredocAndUnknownRMFlag(t *testing.T) {
	for _, in := range []string{"cat <<EOF\nx\nEOF\n", "rm --one-file-system x"} {
		got := Transform(in)
		assert.True(t, got.Denied)

	}
}

func TestTransformFailsOpenOnParseError(t *testing.T) {
	in := "if"
	got := Transform(in)
	require.False(t, got.Denied || got.Command != in)

}
