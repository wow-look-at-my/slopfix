package treecomments

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userscriptSrc = "// ==UserScript==\n" +
	"// @name         ts0 sample userscript\n" +
	"// @version      1.0\n" +
	"// @description  Bundled by ts0: iife + globalName + preserveHeader\n" +
	"// @match        https://example.com/*\n" +
	"// @grant        none\n" +
	"// ==/UserScript==\n" +
	"\n" +
	"// The header above is metadata the userscript manager reads.\n" +
	"export const x = 1;\n"

// A userscript manager reads the metadata block from the installed file, so
// no rule may read it as prose.
func TestUserscriptMetadataIsNotAComment(t *testing.T) {
	comments := Extract("main.user.ts", userscriptSrc)
	require.Len(t, comments, 1)
	assert.True(t, strings.HasPrefix(comments[0].Text, "// The header above"))
}

// A block with no closing line is not metadata, so its comments stay prose.
func TestAnUnclosedUserscriptBlockStaysProse(t *testing.T) {
	src := "// ==UserScript==\n// @name x\nexport const x = 1;\n"
	assert.Len(t, Extract("main.user.ts", src), 2)
}
