package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/code"
)

// Every grammar gets the same walk, the same measure and the same repair. A
// language that only REPORTS is the failure this pins: the old path repaired Go
// alone, and every other language got a finding nothing could act on.
var languageFixtures = map[string]string{
	"x.go":  "package p\n\n" + essay("//") + "const p = 1\n",
	"x.c":   essay("//") + "int p = 1;\n",
	"x.cc":  essay("//") + "int p = 1;\n",
	"x.rs":  essay("//") + "const P: i32 = 1;\n",
	"x.sh":  "#!/bin/sh\n" + essay("#") + "p=1\n",
	"x.js":  essay("//") + "const p = 1;\n",
	"x.ts":  essay("//") + "const p: number = 1;\n",
	"x.tsx": essay("//") + "const p: number = 1;\n",
}

// essay is a comment far past anything a single declaration can carry.
func essay(marker string) string {
	var b strings.Builder
	for range 6 {
		b.WriteString(marker + " An explanation that runs well past the declaration below it.\n")
	}
	return b.String()
}

func TestEveryGrammarReportsAndRepairs(t *testing.T) {
	for name, src := range languageFixtures {
		require.True(t, Parsed(name), "%s: the rule does not claim this file", name)

		// An accepted grammar can still yield a broken tree. Java did.
		_, parses := treeBlocks(languageFor(name), src)
		require.True(t, parses, "%s: the grammar does not parse its own fixture", name)

		hits := Check(name, src)
		require.NotEmpty(t, hits, "%s: reports nothing", name)
		assert.True(t, hits[0].Repairable, "%s: reports a finding the repair cannot act on", name)

		fixed, changed := Fix(name, src)
		assert.True(t, changed, "%s: the repair did nothing", name)
		assert.Less(t, len(fixed), len(src), "%s: the repair did not shorten the file", name)
		assert.Empty(t, Check(name, fixed), "%s: still over after the repair", name)
	}
}

// The repair keeps the opening sentence, in every language. A block cut to
// nothing is a worse edit than a block left long.
func TestTheRepairKeepsTheOpeningInEveryGrammar(t *testing.T) {
	for name, src := range languageFixtures {
		fixed, changed := Fix(name, src)
		require.True(t, changed, name)
		assert.Contains(t, fixed, "An explanation that runs well past", name)
	}
}

// Every extension the rule claims is covered above. A grammar added without a
// fixture is a language nobody proved the repair works on.
func TestEveryClaimedExtensionHasAFixture(t *testing.T) {
	covered := map[string]bool{}
	for name := range languageFixtures {
		covered[name[strings.LastIndex(name, "."):]] = true
	}
	for _, ext := range code.Extensions() {
		assert.True(t, covered[ext] || sharesAGrammar(ext, covered),
			"%s is claimed by the rule and no fixture exercises it", ext)
	}
}

// sharesAGrammar reports an extension whose grammar a covered extension already
// drives, so a header or an alias needs no fixture of its own.
func sharesAGrammar(ext string, covered map[string]bool) bool {
	for other := range covered {
		mine, theirs := code.LanguageFor("x"+ext), code.LanguageFor("x"+other)
		if mine == nil || theirs == nil {
			continue
		}
		if mine == theirs {
			return true
		}
	}
	return false
}
