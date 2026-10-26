package workflow_test

import _ "embed"

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
			"other-org/tool":              "master",
			"actions/checkout":            "main",
		},
		branches: set.Of(
			"wow-look-at-my/go-toolchain@master",
			"wow-look-at-my/go-toolchain@drop-common-checks",
			"other-org/tool@master",
			"other-org/tool@drop-common-checks",
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
	content := testAFeatureBranchPinIsReportedYml
	found, err := workflow.BranchPins(content, orgRefs())
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, 7, found[0].Line)
	assert.Contains(t, found[0].Detail, "other-org/tool@drop-common-checks")
	assert.Contains(t, found[0].Detail, "not its default branch")
}

func TestTheDefaultBranchATagAndAFullSHAPass(t *testing.T) {
	content := testTheDefaultBranchATagAndAFullSHAPassYml
	refs := orgRefs()
	assert.Empty(t, pinFindings(t, content, refs))
	assert.NotContains(t, refs.asked, "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683", "a full SHA needs no lookup")
}

// An orphan-release tag cut from a branch build is deleted with the branch.
func TestATagCutFromAFeatureBranchIsReported(t *testing.T) {
	content := testATagCutFromAFeatureBranchIsReportedYml
	found := pinFindings(t, content, orgRefs())
	require.Len(t, found, 1)
	assert.Contains(t, found[0], "a build of the branch pi-signoff")
}

func TestAReusableWorkflowAndACompositeStepAreRead(t *testing.T) {
	content := testAReusableWorkflowAndACompositeStepAreReadYml
	assert.Len(t, pinFindings(t, content, orgRefs()), 1)
	composite := testAReusableWorkflowAndACompositeStepAreReadYml2
	assert.Len(t, pinFindings(t, composite, orgRefs()), 1)
}

// The incident: the branch is gone, so the ref names nothing.
func TestARefThatNamesNothingIsReported(t *testing.T) {
	content := testARefThatNamesNothingIsReportedYml
	found := pinFindings(t, content, orgRefs())
	require.Len(t, found, 1)
	assert.Contains(t, found[0], "neither a tag nor a branch")
}

func TestALookupThatFailsIsAnError(t *testing.T) {
	content := testALookupThatFailsIsAnErrorYml
	_, err := workflow.BranchPins(content, orgRefs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "someone/private")
}

// A uses: key outside a job or a step is an input name, not an action.
func TestOnlyAJobOrAStepUsesIsRead(t *testing.T) {
	content := testOnlyAJobOrAStepUsesIsReadYml
	assert.Empty(t, workflow.Uses(content))
	assert.Empty(t, workflow.Uses("jobs: [unbalanced\n"))
}

//go:embed testdata/test-a-feature-branch-pin-is-reported.yml
var testAFeatureBranchPinIsReportedYml string

//go:embed testdata/test-the-default-branch-a-tag-and-a-full-sha-pass.yml
var testTheDefaultBranchATagAndAFullSHAPassYml string

//go:embed testdata/test-a-tag-cut-from-a-feature-branch-is-reported.yml
var testATagCutFromAFeatureBranchIsReportedYml string

//go:embed testdata/test-a-reusable-workflow-and-a-composite-step-are-read.yml
var testAReusableWorkflowAndACompositeStepAreReadYml string

//go:embed testdata/test-a-reusable-workflow-and-a-composite-step-are-read-2.yml
var testAReusableWorkflowAndACompositeStepAreReadYml2 string

//go:embed testdata/test-a-ref-that-names-nothing-is-reported.yml
var testARefThatNamesNothingIsReportedYml string

//go:embed testdata/test-a-lookup-that-fails-is-an-error.yml
var testALookupThatFailsIsAnErrorYml string

//go:embed testdata/test-only-a-job-or-a-step-uses-is-read.yml
var testOnlyAJobOrAStepUsesIsReadYml string
