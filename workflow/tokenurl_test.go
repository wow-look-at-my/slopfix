package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

const tokenURLLine = `git config --global url."https://${PRIVATE_ORG_REPO_READ}@github.com/wow-look-at-my/".insteadOf "https://github.com/wow-look-at-my/"`

const tokenHelperLine = `git config --global credential.https://github.com.helper '!f() { echo username=x-access-token; echo password=$PRIVATE_ORG_REPO_READ; }; f'`

func tokenURLFindings(content string) []ste.Finding {
	var out []ste.Finding
	for _, f := range workflow.Check(content) {
		if f.ID == workflow.IDTokenInGitURL {
			out = append(out, f)
		}
	}
	return out
}

func tokenURLJob(run string) string {
	return "on: workflow_dispatch\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - env:\n          PRIVATE_ORG_REPO_READ: ${{ secrets.PRIVATE_ORG_REPO_READ }}\n        run: " + run
}

func repairedTokenURL(content string) string {
	return workflow.Fix(content, func(id string) bool { return id == workflow.IDTokenInGitURL }).Text
}

func TestATokenInAGitURLIsReported(t *testing.T) {
	found := tokenURLFindings(tokenURLJob(tokenURLLine + "\n"))
	require.Len(t, found, 1)
	assert.Equal(t, 8, found[0].Line)
	assert.Equal(t, "$PRIVATE_ORG_REPO_READ", found[0].Detail)
	assert.Contains(t, found[0].Rule, "could not read Username")
	assert.Contains(t, found[0].Fix, "credential helper")
}

func TestTheSingleLineRepairWritesTheHelper(t *testing.T) {
	fixed := repairedTokenURL(tokenURLJob(tokenURLLine + "\n"))
	assert.Equal(t, tokenURLJob(tokenHelperLine+"\n"), fixed)
	assert.Empty(t, tokenURLFindings(fixed))
}

func TestTheBlockRepairKeepsTheIndentAndTheOtherLines(t *testing.T) {
	block := "|\n          set -e\n          " + tokenURLLine + "\n          go build ./...\n"
	want := "|\n          set -e\n          " + tokenHelperLine + "\n          go build ./...\n"
	fixed := repairedTokenURL(tokenURLJob(block))
	assert.Equal(t, tokenURLJob(want), fixed)
	assert.Empty(t, tokenURLFindings(fixed))
}

func TestTheHelperReadsTheVariableTheOldLineUsed(t *testing.T) {
	for old, variable := range map[string]string{
		`git config --global url."https://$GH_TOKEN@github.com/".insteadOf https://github.com/`:     "GH_TOKEN",
		`git config --global "url.https://${GH_TOKEN}@github.com/.insteadOf" "https://github.com/"`: "GH_TOKEN",
		`git config url.https://${GH_TOKEN}@github.com/.insteadOf https://github.com/`:              "GH_TOKEN",
	} {
		fixed := repairedTokenURL(tokenURLJob(old + "\n"))
		assert.Contains(t, fixed, "echo password=$"+variable+"; }; f'", old)
		assert.NotContains(t, fixed, "insteadOf", old)
		assert.Empty(t, tokenURLFindings(fixed), old)
	}
}

// The scope flag stays, and the rest of the list stays.
func TestTheRepairKeepsTheScopeAndTheRestOfTheList(t *testing.T) {
	old := `git config --system url."https://${GH_TOKEN}@github.com/".insteadOf https://github.com/ && git ls-remote origin`
	fixed := repairedTokenURL(tokenURLJob(old + "\n"))
	assert.Contains(t, fixed, "run: git config --system credential.https://github.com.helper '!f() { echo username=x-access-token; echo password=$GH_TOKEN; }; f' && git ls-remote origin\n")
}

func TestAnInsteadOfWithNoTokenIsNotReported(t *testing.T) {
	for _, run := range []string{
		`git config --global url."https://github.com/".insteadOf "git@github.com:"`,
		`git config --global url."git@github.com:".insteadOf https://github.com/`,
		`git config --global url."https://github.com/".insteadOf "ssh://git@github.com/"`,
		`git config --global url."https://x-access-token@github.com/".insteadOf https://github.com/`,
	} {
		assert.Empty(t, tokenURLFindings(tokenURLJob(run+"\n")), run)
	}
}

func TestAUserAndPasswordURLIsNotReported(t *testing.T) {
	for _, run := range []string{
		`git config --global url."https://x-access-token:${GH_TOKEN}@github.com/".insteadOf https://github.com/`,
		`git config --global url."https://x-access-token:$GH_TOKEN@github.com/".insteadOf https://github.com/`,
	} {
		assert.Empty(t, tokenURLFindings(tokenURLJob(run+"\n")), run)
	}
}

func TestAnotherHostIsNotReported(t *testing.T) {
	for _, run := range []string{
		`git config --global url."https://${GITLAB_TOKEN}@gitlab.com/".insteadOf https://gitlab.com/`,
		`git config --global url."https://${GH_TOKEN}@github.company.com/".insteadOf https://github.company.com/`,
		`git config --global url."https://${GH_TOKEN}@ghe.example.com/".insteadOf https://ghe.example.com/`,
	} {
		assert.Empty(t, tokenURLFindings(tokenURLJob(run+"\n")), run)
	}
}

func TestOtherGitCommandsAreNotReported(t *testing.T) {
	for _, run := range []string{
		`git config --global --unset url."https://${GH_TOKEN}@github.com/".insteadOf`,
		`echo url."https://${GH_TOKEN}@github.com/".insteadOf`,
		`git remote set-url origin "https://${GH_TOKEN}@github.com/o/r"`,
		`git config --global url."https://${{ secrets.TOKEN }}@github.com/".insteadOf https://github.com/`,
	} {
		assert.Empty(t, tokenURLFindings(tokenURLJob(run+"\n")), run)
	}
}

// A helper that git starts reads only the exported environment.
func TestAVariableTheScriptAssignsIsNotReported(t *testing.T) {
	block := "|\n          GH_TOKEN=\"$(cat token)\"\n          git config --global url.\"https://${GH_TOKEN}@github.com/\".insteadOf https://github.com/\n"
	assert.Empty(t, tokenURLFindings(tokenURLJob(block)))
}

func TestACompositeActionStepIsReported(t *testing.T) {
	content := "name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n      run: |\n        git config --global url.\"https://${GH_TOKEN}@github.com/\".insteadOf https://github.com/\n"
	found := tokenURLFindings(content)
	require.Len(t, found, 1)
	assert.Equal(t, 7, found[0].Line)
	fixed := repairedTokenURL(content)
	assert.Contains(t, fixed, "        git config --global credential.https://github.com.helper '!f() { echo username=x-access-token; echo password=$GH_TOKEN; }; f'\n")
	assert.Empty(t, tokenURLFindings(fixed))
}

func TestTwoRewritesOnOneLineAreBothRepaired(t *testing.T) {
	old := `git config --global url."https://${A}@github.com/a/".insteadOf https://github.com/a/; git config --global url."https://${B}@github.com/b/".insteadOf https://github.com/b/`
	require.Len(t, tokenURLFindings(tokenURLJob(old+"\n")), 2)
	fixed := repairedTokenURL(tokenURLJob(old + "\n"))
	assert.Empty(t, tokenURLFindings(fixed))
	assert.Contains(t, fixed, "password=$A; }; f'; git config --global credential")
	assert.Contains(t, fixed, "password=$B; }; f'\n")
}
