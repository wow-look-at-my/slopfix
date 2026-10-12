package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// The suite a repair once broke in xml-validator. The bash grammar read the
// `#` in `&#0;` as a comment, and the cut left `<r>a&` behind.
const glued = "tests:\n" +
	"\t- desc: a document with a NUL character reference is valid\n" +
	"\t  inputs:\n" +
	"\t\tfiles:\n" +
	"\t\t\tnul.xml: |\n" +
	"\t\t\t\t<?xml version=\"1.1\"?>\n" +
	"\t\t\t\t<r>a&#0;b</r>\n" +
	"\t\t\t\t<a>1&#0;2</a>\n" +
	"\t\t\t\t<b at=\"&#x0;\">after</b>\n"

func TestARepairKeepsAGluedHash(t *testing.T) {
	req := slopfix.Request{Path: "cli/dats/nul-char-ref.dats", Content: glued, MaxCommentLines: tombstones.DefaultMaxCommentLines}
	assert.Equal(t, glued, slopfix.Fix(req).Text)
	assert.Empty(t, slopfix.Report(req).Findings)
}

// A `#` after a blank still opens a comment, and the rules still read it.
func TestAHashAfterABlankIsStillAComment(t *testing.T) {
	src := "tests:\n\t# The suite checks 3 things about the reference.\n\t- desc: one\n"
	req := slopfix.Request{Path: "suite.dats", Content: src, MaxCommentLines: tombstones.DefaultMaxCommentLines}
	assert.NotEqual(t, src, slopfix.Fix(req).Text)
}
