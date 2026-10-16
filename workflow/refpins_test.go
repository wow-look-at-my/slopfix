package workflow_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// fakeRefs answers from tables, and counts each question so a test can see a
// full SHA is never asked about.
type fakeRefs struct {
	defaults map[string]string
	branches set.Set[string]
	tags     set.Set[string]
	asked    []string
}

func (f *fakeRefs) DefaultBranch(repo string) (string, error) {
	def, ok := f.defaults[repo]
	if !ok {
		return "", errors.New("no access to " + repo)
	}
	return def, nil
}

func (f *fakeRefs) Kind(repo, ref string) (workflow.RefKind, error) {
	f.asked = append(f.asked, repo+"@"+ref)
	if _, ok := f.defaults[repo]; !ok {
		return workflow.RefMissing, errors.New("no access to " + repo)
	}
	switch {
	case f.branches.Contains(repo + "@" + ref):
		return workflow.RefBranch, nil
	case f.tags.Contains(repo + "@" + ref):
		return workflow.RefTag, nil
	}
	return workflow.RefMissing, nil
}

func orgRefs() *fakeRefs {
	return &fakeRefs{
		defaults: map[string]string{
			"wow-look-at-my/go-toolchain": "master",
			"wow-look-at-my/slopfix":      "master",
			"wow-look-at-my/actions":      "master",
			"wow-look-at-my/buildhost":    "master",
			"actions/checkout":            "main",
		},
		branches: set.Of(
			"wow-look-at-my/go-toolchain@master",
			"wow-look-at-my/go-toolchain@drop-common-checks",
			"wow-look-at-my/slopfix@master",
			"wow-look-at-my/actions@master",
			"wow-look-at-my/actions@pi-signoff",
			"wow-look-at-my/buildhost@master",
			"actions/checkout@main",
		),
		tags: set.Of(
			"actions/checkout@v4",
			"wow-look-at-my/actions@secret-server#latest",
			"wow-look-at-my/actions@secret-server/pi-signoff#1",
			"wow-look-at-my/actions@secret-server/master#latest",
		),
	}
}

func pinFindings(t *testing.T, content string, refs workflow.Refs) []string {
	t.Helper()
	found, err := workflow.BranchPins(content, refs)
	require.NoError(t, err)
	var out []string
	for _, f := range found {
		assert.Equal(t, workflow.IDBranchPin, f.ID)
		out = append(out, f.Detail)
	}
	return out
}

func TestAFeatureBranchPinIsReported(t *testing.T) {
	content := "on: {push: {branches: ['**']}}\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - uses: wow-look-at-my/go-toolchain@drop-common-checks\n"
	found, err := workflow.BranchPins(content, orgRefs())
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, 7, found[0].Line)
	assert.Contains(t, found[0].Detail, "wow-look-at-my/go-toolchain@drop-common-checks")
	assert.Contains(t, found[0].Detail, "not its default branch")
}

func TestTheDefaultBranchATagAndAFullSHAPass(t *testing.T) {
	content := "jobs:\n  build:\n    steps:\n" +
		"      - uses: wow-look-at-my/go-toolchain@master\n" +
		"      - uses: wow-look-at-my/slopfix@master\n" +
		"      - uses: actions/checkout@v4\n" +
		"      - uses: actions/checkout@main\n" +
		"      - uses: wow-look-at-my/actions@secret-server#latest\n" +
		"      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683\n" +
		"      - uses: ./local-action\n" +
		"      - uses: docker://alpine:3.20\n" +
		"      - uses: ${{ matrix.action }}\n"
	refs := orgRefs()
	assert.Empty(t, pinFindings(t, content, refs))
	assert.NotContains(t, refs.asked, "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683", "a full SHA needs no lookup")
}

// An orphan-release tag cut from a branch build is deleted with the branch.
func TestATagCutFromAFeatureBranchIsReported(t *testing.T) {
	content := "runs:\n  using: composite\n  steps:\n    - uses: wow-look-at-my/actions@secret-server/pi-signoff#1\n    - uses: wow-look-at-my/actions@secret-server/master#latest\n"
	found := pinFindings(t, content, orgRefs())
	require.Len(t, found, 1)
	assert.Contains(t, found[0], "a build of the branch pi-signoff")
}

func TestAReusableWorkflowAndACompositeStepAreRead(t *testing.T) {
	content := "jobs:\n  publish:\n    uses: wow-look-at-my/go-toolchain/.github/workflows/build.yml@drop-common-checks\n"
	assert.Len(t, pinFindings(t, content, orgRefs()), 1)
	composite := "runs:\n  using: composite\n  steps:\n    - uses: wow-look-at-my/buildhost/.github/actions/buildhost-download@master\n    - uses: wow-look-at-my/go-toolchain@drop-common-checks\n"
	assert.Len(t, pinFindings(t, composite, orgRefs()), 1)
}

// The incident: the branch is gone, so the ref names nothing.
func TestARefThatNamesNothingIsReported(t *testing.T) {
	content := "jobs:\n  build:\n    steps:\n      - uses: wow-look-at-my/go-toolchain@deleted-branch\n"
	found := pinFindings(t, content, orgRefs())
	require.Len(t, found, 1)
	assert.Contains(t, found[0], "neither a tag nor a branch")
}

func TestALookupThatFailsIsAnError(t *testing.T) {
	content := "jobs:\n  build:\n    steps:\n      - uses: someone/private@v1\n"
	_, err := workflow.BranchPins(content, orgRefs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "someone/private")
}

// A uses: key outside a job or a step is an input name, not an action.
func TestOnlyAJobOrAStepUsesIsRead(t *testing.T) {
	content := "jobs:\n  build:\n    steps:\n      - with:\n          uses: wow-look-at-my/go-toolchain@drop-common-checks\n"
	assert.Empty(t, workflow.Uses(content))
	assert.Empty(t, workflow.Uses("jobs: [unbalanced\n"))
}
