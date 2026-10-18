package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/token-in-git-url: a git config rewrite that puts a token in the user
// part of a github.com URL. The repair sets a credential helper that reads the
// token from the environment.
func init() {
	job := func(step string) string {
		return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n" + step
	}
	env := "      - env:\n          PRIVATE_ORG_REPO_READ: ${{ secrets.PRIVATE_ORG_REPO_READ }}\n"
	helper := "git config --global credential.https://github.com.helper '!f() { echo username=x-access-token; echo password=$PRIVATE_ORG_REPO_READ; }; f'"
	RegisterRule(RuleSpec{
		ID:       workflow.IDTokenInGitURL,
		Category: RuleWorkflow,
		Detect:   detectTokenInGitURL,
		Autofix:  autofixTokenInGitURL,
		Cases: []RuleCase{
			{
				Name: "yaml/token-in-git-url-single-line", Path: ".github/workflows/ci.yml",
				Text: job(env + "        run: git config --global url.\"https://${PRIVATE_ORG_REPO_READ}@github.com/wow-look-at-my/\".insteadOf \"https://github.com/wow-look-at-my/\"\n"),
				Want: job(env + "        run: " + helper + "\n"),
			},
			{
				Name: "yaml/token-in-git-url-block", Path: ".github/workflows/ci.yml",
				Text: job(env + "        run: |\n          set -e\n" +
					"          git config --global url.\"https://${PRIVATE_ORG_REPO_READ}@github.com/wow-look-at-my/\".insteadOf \"https://github.com/wow-look-at-my/\"\n" +
					"          go build ./...\n"),
				Want: job(env + "        run: |\n          set -e\n          " + helper + "\n          go build ./...\n"),
			},
			{
				Name: "yaml/token-in-git-url-access-token", Path: ".github/workflows/ci.yml",
				Text: job(env + "        run: |\n" +
					"          git config --global url.\"https://x-access-token:$PRIVATE_ORG_REPO_READ@github.com/\".insteadOf https://github.com/\n"),
				Want: job(env + "        run: |\n          " + helper + "\n"),
			},
		},
	})
}

// detectTokenInGitURL answers each git config rewrite that holds a token in the URL.
func detectTokenInGitURL(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDTokenInGitURL)
}

// autofixTokenInGitURL writes the credential helper in place of each rewrite.
func autofixTokenInGitURL(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDTokenInGitURL)
}
