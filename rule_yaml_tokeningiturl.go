package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/token-in-git-url: a git config rewrite that puts a token in the user
// part of a github.com URL with no password. The repair writes x-access-token
// as the user, which leaves the token as the password.
func init() {
	job := func(step string) string {
		return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n" + step
	}
	env := "      - env:\n          PRIVATE_ORG_REPO_READ: ${{ secrets.PRIVATE_ORG_REPO_READ }}\n"
	rewrite := func(user string) string {
		return "git config --global url.\"https://" + user + "${PRIVATE_ORG_REPO_READ}@github.com/wow-look-at-my/\".insteadOf \"https://github.com/wow-look-at-my/\""
	}
	RegisterRule(RuleSpec{
		ID:       workflow.IDTokenInGitURL,
		Category: RuleWorkflow,
		Detect:   detectTokenInGitURL,
		Autofix:  autofixTokenInGitURL,
		Cases: []RuleCase{
			{
				Name: "yaml/token-in-git-url-single-line", Path: ".github/workflows/ci.yml",
				Text: job(env + "        run: " + rewrite("") + "\n"),
				Want: job(env + "        run: " + rewrite("x-access-token:") + "\n"),
			},
			{
				Name: "yaml/token-in-git-url-block", Path: ".github/workflows/ci.yml",
				Text: job(env + "        run: |\n          set -e\n          " + rewrite("") + "\n          go build ./...\n"),
				Want: job(env + "        run: |\n          set -e\n          " + rewrite("x-access-token:") + "\n          go build ./...\n"),
			},
		},
	})
}

// detectTokenInGitURL answers each git config rewrite that holds a token in the URL.
func detectTokenInGitURL(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDTokenInGitURL)
}

// autofixTokenInGitURL writes x-access-token as the user of each rewrite.
func autofixTokenInGitURL(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDTokenInGitURL)
}
