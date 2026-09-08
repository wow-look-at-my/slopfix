package commentlength

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func itoa(n int) string { return strconv.Itoa(n) }

func boolText(b bool) string { return strconv.FormatBool(b) }

// Every grammar gets the same walk, the same measure and the same repair. A
// language that only REPORTS is the failure this pins: the old path repaired Go
// alone, and every other language got a finding nothing could act on.
var languageFixtures = map[string]string{
	"x.go":   "package p\n\n" + essay("//") + "const p = 1\n",
	"x.c":    essay("//") + "int p = 1;\n",
	"x.cc":   essay("//") + "int p = 1;\n",
	"x.rs":   essay("//") + "const P: i32 = 1;\n",
	"x.sh":   "#!/bin/sh\n" + essay("#") + "p=1\n",
	"x.java": "class C {\n" + essay("  //") + "  int p = 1;\n}\n",
}

// essay is a comment far past anything a single declaration can carry.
func essay(marker string) string {
	var b strings.Builder
	for range 6 {
		b.WriteString(marker + " An explanation that runs on well past the declaration below it,\n")
	}
	return b.String()
}

func TestEveryGrammarReportsAndRepairs(t *testing.T) {
	var report strings.Builder
	for name, src := range languageFixtures {
		require.True(t, Parsed(name), "%s: the rule does not claim this file", name)

		bs := blocks(name, src)
		_, parses := treeBlocks(languageFor(name), src)
		report.WriteString(name + " parses=" + boolText(parses) + " blocks=" + itoa(len(bs)))
		for _, b := range bs {
			tell, over := judge(b)
			report.WriteString(" [" + itoa(b.start+1) + " code=" + itoa(b.codeLines) + "l/" +
				itoa(b.codeChars) + "c over=" + boolText(over) + " " + tell + "]")
		}
		report.WriteString("\n")
		_ = os.WriteFile(filepath.Join(os.TempDir(), "lang-report.txt"), []byte(report.String()), 0o644)

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
		assert.Contains(t, fixed, "An explanation that runs on well past", name)
	}
}

// Every extension the rule claims is covered above. A grammar added without a
// fixture is a language nobody proved the repair works on.
func TestEveryClaimedExtensionHasAFixture(t *testing.T) {
	covered := map[string]bool{}
	for name := range languageFixtures {
		covered[name[strings.LastIndex(name, "."):]] = true
	}
	for ext := range grammars {
		assert.True(t, covered[ext] || sharesAGrammar(ext, covered),
			"%s is claimed by the rule and no fixture exercises it", ext)
	}
}

// sharesAGrammar reports an extension whose grammar a covered extension already
// drives, so a header or an alias needs no fixture of its own.
func sharesAGrammar(ext string, covered map[string]bool) bool {
	for other := range covered {
		if grammars[ext] == nil || grammars[other] == nil {
			continue
		}
		if grammars[ext]() == grammars[other]() {
			return true
		}
	}
	return false
}
