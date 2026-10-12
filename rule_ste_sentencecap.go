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
