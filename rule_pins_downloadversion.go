package slopfix

import (
	"github.com/wow-look-at-my/slopfix/pins"
	"github.com/wow-look-at-my/slopfix/ste"
)

// pins/download-version: a download URL that names an exact release breaks when
// that release is gone. The repair deletes the v query parameter, so the URL
// serves the newest published build on the default branch.
func init() {
	RegisterRule(RuleSpec{
		ID:       pins.ID,
		Category: RulePins,
		Detect:   detectDownloadVersion,
		Autofix:  autofixDownloadVersion,
		// The URL is joined from parts, so no rule reads this file as a pinned link.
		Cases: []RuleCase{{Name: pins.ID, Path: "fetch.sh", Text: "curl https://dl.pazer.build/slopfix?" + "v=1.2.3\n"}},
	})
}

// detectDownloadVersion answers every pinned download URL this rule reports.
func detectDownloadVersion(c RuleCase) []ste.Finding {
	return caseFindings(c, pins.ID)
}

// autofixDownloadVersion deletes the v query parameter from the URL.
func autofixDownloadVersion(c RuleCase) RuleCase {
	return caseAutofix(c, pins.ID)
}
