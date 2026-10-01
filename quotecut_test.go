package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
)

// A document line opening on a quoted phrase was once cut to "When." and the
// rule it stated was gone. Whatever a repair does to it, the quote survives.
func TestADocumentSentenceWithAQuoteKeepsIt(t *testing.T) {
	src := "# Git\n\n" +
		"When the user says \"clean up the commit message\", they mean amend the LOCAL commit message, not fetch/reset/destroy their work.\n"
	for _, rule := range slopfix.AllRules {
		t.Run(string(rule), func(t *testing.T) {
			repair := slopfix.Fix(slopfix.Request{Path: "CLAUDE.git.md", Content: src, Rules: []slopfix.Rule{rule}})
			assert.Contains(t, repair.Text, "\"clean up the commit message\"", repair.Text)
		})
	}
	repair := slopfix.Fix(slopfix.Request{Path: "CLAUDE.git.md", Content: src})
	assert.Contains(t, repair.Text, "\"clean up the commit message\"", repair.Text)
}
