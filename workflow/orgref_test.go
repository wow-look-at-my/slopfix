package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

func orgRefFindings(content string) []ste.Finding {
	var out []ste.Finding
	for _, f := range workflow.Check(content) {
		if f.ID == workflow.IDOrgActionRef {
			out = append(out, f)
		}
	}
	return out
}

func steps(uses ...string) string {
	out := "on: workflow_dispatch\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n"
	for _, u := range uses {
		out += "      - uses: " + u + "\n"
	}
	return out
}

func TestAnOrgActionOffMasterIsReported(t *testing.T) {
	for _, uses := range []string{
		"wow-look-at-my/go-toolchain@drop-common-checks",
		"wow-look-at-my/go-toolchain@v1",
		"Wow-Look-At-My/go-toolchain@main",
		"wow-look-at-my/buildhost/.github/actions/buildhost-publish@feature",
		"'wow-look-at-my/slopfix@claude/x'",
		"wow-look-at-my/actions@typescript",
		"wow-look-at-my/actions@typescript#3",
		"wow-look-at-my/go-toolchain@0123456789abcdef0123456789abcdef01234567",
	} {
		found := orgRefFindings(steps(uses))
		require.Len(t, found, 1, uses)
		assert.Equal(t, 6, found[0].Line, uses)
	}
}

func TestAReusableWorkflowOffMasterIsReported(t *testing.T) {
	content := "on: workflow_dispatch\njobs:\n  preview:\n    uses: wow-look-at-my/buildhost/.github/workflows/preview.yml@wip\n"
	found := orgRefFindings(content)
	require.Len(t, found, 1)
	assert.Equal(t, 4, found[0].Line)
}

func TestAnOrgActionOnMasterPasses(t *testing.T) {
	content := steps(
		"wow-look-at-my/go-toolchain@master",
		"wow-look-at-my/actions@typescript#latest",
		"wow-look-at-my/buildhost/.github/actions/buildhost-publish@master",
		"actions/checkout@v4",
		"PazerOP/pr-preview-action@v1",
		"./local-action",
		"docker://alpine:3",
	)
	assert.Empty(t, orgRefFindings(content))
	assert.Empty(t, orgRefFindings("runs:\n  using: composite\n  steps:\n    - uses: wow-look-at-my/dats@master\n"))
}

func TestTheRepairPointsAtMaster(t *testing.T) {
	cases := map[string]string{
		"wow-look-at-my/go-toolchain@drop-common-checks":     "wow-look-at-my/go-toolchain@master",
		"'wow-look-at-my/slopfix@v1' # gate":                 "'wow-look-at-my/slopfix@master' # gate",
		"wow-look-at-my/actions@typescript":                  "wow-look-at-my/actions@typescript#latest",
		"wow-look-at-my/actions@typescript#3":                "wow-look-at-my/actions@typescript#latest",
		"wow-look-at-my/buildhost/.github/actions/x@feature": "wow-look-at-my/buildhost/.github/actions/x@master",
	}
	for uses, want := range cases {
		out := repaired(t, steps(uses))
		assert.Equal(t, steps(want), out, uses)
		assert.Empty(t, orgRefFindings(out), uses)
	}
}
