package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/sentence-length: a sentence over the STE word cap. The repair divides it
// at a clause boundary, and between words only where no boundary reads.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDSentenceCap,
		Category: RuleSTE,
		Detect:   detectSentenceCap,
		Autofix:  autofixSentenceCap,
		Cases: []RuleCase{
			{Name: ste.IDSentenceCap, Path: "x.md", Text: "This sentence carries far more words than any reader can hold in mind at one time and it keeps going well past the cap of the rule today.\n"},
			// go-toolchain docs/CI.md: a stray backtick closes a path with no opener, and the code spans after it stay whole.
			{Name: ste.IDSentenceCap + " with a stray backtick", Path: "docs/CI.md", Text: "Their jobs run .github/dats-fixtures/cache-profile.dats` asserts the tripwires themselves, run by the dats action in `host-build` the same way `same.dats` and `smoke.dats`.\n"},
			// sglang .github/scripts/ghcr-image-retention.ts: "an image a week old" is the subject after the colon, so no other subject is put in front of it.
			{Name: ste.IDSentenceCap + " with a measure after the subject", Path: "x.md", Text: "Ranked over every default-branch image, so the retention count is what bounds the history: an image a week old does not hold a slot that then falls to the newest of the images the age guard has already spared.\n"},
			// go-ipc .github/workflows/ci.yml: "the C and Python ones" is one noun phrase, so no division lands inside it.
			{Name: ste.IDSentenceCap + " around a gapped clause", Path: "x.md", Text: "Linux builds and tests the C, C++ and Python implementations and their peers first, macOS the C and Python ones, and Windows runs the interop matrix with the Go peer alone.\n"},
			// claude-host Dockerfile: "and by B to Q" names a second actor for the participle.
			{Name: ste.IDSentenceCap + " with a second actor", Path: "x.md", Text: "Shared stage used by Dockerfile.worker to pre-install the static code lint extension and by the job scheduler (below) to seed the private build cache that it serves.\n"},
			// claude-host push_reminder_test.go: a dash aside before "so".
			{Name: ste.IDSentenceCap + " with a dash aside before so", Path: "x.md", Text: "A stale index is a different fault than a stale cache -- it is not on disk at all -- so it has to be rebuilt even when every file is on the server.\n"},
			// claude-host web-src/session-permissions.ts: ", each with" gives each item what follows.
			{Name: ste.IDSentenceCap + " before each with", Path: "x.md", Text: "Lists what the client asked for and was denied, each with a one-step way to grant it going forward instead of searching through the access table above.\n"},
			// claude-host github_permissions_test.go: a bare verb after the comma shares the subject and its modal, so both are named again.
			{Name: ste.IDSentenceCap + " before a verb that shares the modal", Path: "x.md", Text: "A feature added to the registry after that rule was written must default to blocked, refuse the open default, and stay that way for every running job.\n"},
			// claude-host docs/session-activity.md: a "that" clause after a list is another item of it.
			{Name: ste.IDSentenceCap + " before a that clause in a list", Path: "x.md", Text: "- `queue_order_test.go` covers the priority order, the two-pass rule, the head-only window, case-folding, the size limit, that a failed fetch keeps the previous value.\n"},
			// sglang .github/workflows/fork-ci.yml: a comment in a run: script is prose, and divides like any comment.
			workflowCase("sentence-length in a run: script", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n"+
				"          ON_DEFAULT=false\n\n"+
				"          # The image installs only this fork's kernel build, so a PyPI wheel of\n"+
				"          # the same version does not count: it lacks the fork's ops and archs.\n"+
				"          KERNEL_AVAILABLE=false\n"),
		},
	})
}

// detectSentenceCap answers each sentence over the word cap that the case
// carries.
func detectSentenceCap(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDSentenceCap)
}

// autofixSentenceCap divides the over-long sentence at a clause boundary, and
// answers the case without the finding.
func autofixSentenceCap(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDSentenceCap)
}
