package workflow_test

import _ "embed"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
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
	out := stepsYml
	for _, u := range uses {
		out += "      - uses: " + u + "\n"
	}
	return out
}

func TestAnOrgActionOffMasterIsReported(t *testing.T) {
	orgAPI(t)
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
	content := testAReusableWorkflowOffMasterIsReportedYml
	found := orgRefFindings(content)
	require.Len(t, found, 1)
	assert.Equal(t, 4, found[0].Line)
}

// orgAPI serves default branches, and an lkgb tag for each repository that publishes one.
func orgAPI(t *testing.T) {
	t.Helper()
	defaults := map[string]string{
		"go-toolchain":          "master",
		"slopfix":               "master",
		"buildhost":             "master",
		"dats":                  "master",
		"actions":               "master",
		"static-pkgsite-action": "v1",
	}
	lkgb := set.Of[string]("go-toolchain", "slopfix", "buildhost", "dats", "actions")
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/repos/wow-look-at-my/"), "/")
		switch {
		case rest == "" && defaults[repo] != "":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"default_branch": defaults[repo]}))
		case rest == "git/ref/tags/lkgb" && lkgb.Contains(repo):
			fmt.Fprint(w, `{"ref":"refs/tags/lkgb"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)
	t.Setenv("GITHUB_API_URL", api.URL)
	t.Setenv("GITHUB_TOKEN", "fixture")
}

func TestAnOrgActionAtLKGBPasses(t *testing.T) {
	orgAPI(t)
	content := steps(
		"wow-look-at-my/go-toolchain@lkgb",
		"wow-look-at-my/actions@typescript#latest",
		"wow-look-at-my/buildhost/.github/actions/buildhost-publish@lkgb",
		"wow-look-at-my/static-pkgsite-action@v1",
		"actions/checkout@v4",
		"PazerOP/pr-preview-action@v1",
		"./local-action",
		"docker://alpine:3",
	)
	assert.Empty(t, orgRefFindings(content))
	assert.Empty(t, orgRefFindings(testAnOrgActionAtLKGBPassesYml))
}

// A red master reaches every caller that names it. lkgb is the last master commit that passed CI.
func TestTheDefaultBranchOfARepositoryWithLKGBIsReported(t *testing.T) {
	orgAPI(t)
	for _, uses := range []string{
		"wow-look-at-my/go-toolchain@master",
		"wow-look-at-my/buildhost/.github/actions/buildhost-publish@master",
		"wow-look-at-my/actions@master",
	} {
		require.Len(t, orgRefFindings(steps(uses)), 1, uses)
	}
	assert.Len(t, orgRefFindings(testTheDefaultBranchOfARepositoryWithLKGBIsReportedYml), 1)
}

func TestTheRepairPointsAtLKGB(t *testing.T) {
	orgAPI(t)
	cases := map[string]string{
		"wow-look-at-my/go-toolchain@v1":                     "wow-look-at-my/go-toolchain@lkgb",
		"wow-look-at-my/go-toolchain@master":                 "wow-look-at-my/go-toolchain@lkgb",
		"wow-look-at-my/go-toolchain@drop-common-checks":     "wow-look-at-my/go-toolchain@lkgb",
		"'wow-look-at-my/slopfix@v1' # gate":                 "'wow-look-at-my/slopfix@lkgb' # gate",
		"wow-look-at-my/actions@master":                      "wow-look-at-my/actions@lkgb",
		"wow-look-at-my/actions@typescript":                  "wow-look-at-my/actions@typescript#latest",
		"wow-look-at-my/actions@typescript#3":                "wow-look-at-my/actions@typescript#latest",
		"wow-look-at-my/buildhost/.github/actions/x@feature": "wow-look-at-my/buildhost/.github/actions/x@lkgb",
	}
	for uses, want := range cases {
		out := repaired(t, steps(uses))
		assert.Equal(t, steps(want), out, uses)
		assert.Empty(t, orgRefFindings(out), uses)
	}
}

// A repository that publishes no lkgb keeps its default branch, because @lkgb would not resolve there.
func TestARepositoryWithoutLKGBKeepsItsDefaultBranch(t *testing.T) {
	orgAPI(t)
	assert.Empty(t, orgRefFindings(steps("wow-look-at-my/static-pkgsite-action@v1")), "the default branch is the tracked ref")
	assert.Equal(t, steps("wow-look-at-my/static-pkgsite-action@v1"), repaired(t, steps("wow-look-at-my/static-pkgsite-action@master")))
	assert.Equal(t, steps("wow-look-at-my/unknown-action@master"), repaired(t, steps("wow-look-at-my/unknown-action@feature")), "an unanswered lookup keeps master")
}

const pinnedSHA = "a9b20e200c7edc8e7bc23fbafbe5ae87eb5f2410"

func TestASHAPinOfTheMonorepoTakesTheTagItsCommentNames(t *testing.T) {
	orgAPI(t)
	cases := map[string]string{
		"wow-look-at-my/actions@" + pinnedSHA + "#latest # cache-upload#latest": "wow-look-at-my/actions@cache-upload#latest # cache-upload#latest",
		"wow-look-at-my/actions@" + pinnedSHA + " # cache-download#latest":      "wow-look-at-my/actions@cache-download#latest # cache-download#latest",
		"wow-look-at-my/go-toolchain@" + pinnedSHA + "#latest":                  "wow-look-at-my/go-toolchain@lkgb",
	}
	for uses, want := range cases {
		require.Len(t, orgRefFindings(steps(uses)), 1, uses)
		out := repaired(t, steps(uses))
		assert.Equal(t, steps(want), out, uses)
		assert.Empty(t, orgRefFindings(out), uses)
	}
}

func TestASHAPinOfTheMonorepoWithNoDirectoryIsNotReported(t *testing.T) {
	for _, uses := range []string{
		"wow-look-at-my/actions@" + pinnedSHA,
		"wow-look-at-my/actions@" + pinnedSHA + " # v4",
		"wow-look-at-my/actions@" + pinnedSHA + " # pinned",
	} {
		assert.Empty(t, orgRefFindings(steps(uses)), uses)
	}
}

// Only a uses: key the runner reads names an action. One under with: is an input value.
func TestAUsesKeyOutsideAJobOrAStepIsNotAnAction(t *testing.T) {
	orgAPI(t)
	content := testAUsesKeyOutsideAJobOrAStepIsNotAnActionYml
	assert.Empty(t, orgRefFindings(content))
	assert.Equal(t, content, repaired(t, content))
}

// The incident: go-toolchain's v1 branch became master and the pin stopped resolving. The repair needs no API answer to write lkgb.
func TestAMovingBranchRepairsToLKGBWhenGitHubCannotAnswer(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(api.Close)
	t.Setenv("GITHUB_API_URL", api.URL)
	t.Setenv("GITHUB_TOKEN", "fixture")
	for uses, want := range map[string]string{
		"wow-look-at-my/go-toolchain@v1":     "wow-look-at-my/go-toolchain@lkgb",
		"wow-look-at-my/go-toolchain@master": "wow-look-at-my/go-toolchain@lkgb",
		"wow-look-at-my/actions@master":      "wow-look-at-my/actions@lkgb",
		"wow-look-at-my/actions@v2":          "wow-look-at-my/actions@lkgb",
	} {
		found := orgRefFindings(steps(uses))
		require.Len(t, found, 1, uses)
		assert.Contains(t, found[0].Detail, "is a branch", uses)
		assert.Equal(t, steps(want), repaired(t, steps(uses)), uses)
	}
}

//go:embed testdata/steps.yml
var stepsYml string

//go:embed testdata/test-a-reusable-workflow-off-master-is-reported.yml
var testAReusableWorkflowOffMasterIsReportedYml string

//go:embed testdata/test-a-uses-key-outside-a-job-or-a-step-is-not-an-action.yml
var testAUsesKeyOutsideAJobOrAStepIsNotAnActionYml string

//go:embed testdata/test-an-org-action-at-lkgb-passes.yml
var testAnOrgActionAtLKGBPassesYml string

//go:embed testdata/test-the-default-branch-of-a-repository-with-lkgb-is-reported.yml
var testTheDefaultBranchOfARepositoryWithLKGBIsReportedYml string
