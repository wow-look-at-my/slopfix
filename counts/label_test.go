package counts

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// resolveItemByRegexp builds the per-label patterns resolveItem replaced. It
// is the oracle: the constant patterns must answer as these do.
func resolveItemByRegexp(content, noun, digits string) string {
	n := regexp.QuoteMeta(noun)
	after := regexp.MustCompile(`(?i)\b` + n + `[ \t]*(?:\([ \t]*` + digits + `[ \t]*\)|` + digits + `[ \t]*[:—–])[ \t]*(the[ \t]+[^,;:.()\n—–]+)`)
	if m := after.FindStringSubmatch(content); m != nil {
		if name := nameOf(m[1]); name != "" {
			return name
		}
	}
	before := regexp.MustCompile(`(?m)^[ \t]*(?:#+[ \t]*)?(?:[-*+][ \t]+)?(?:\d+[.)][ \t]*)?([A-Z][\w'-]*(?:[ \t]+[a-z][\w'-]*){0,3})[ \t]*\([ \t]*(?i:` + n + `)[ \t]*\(?` + digits + `\)?[ \t]*\)`)
	if m := before.FindStringSubmatch(content); m != nil {
		return "the " + lowerFirst(m[1])
	}
	return ""
}

func TestResolveItemAnswersAsThePerLabelPatterns(t *testing.T) {
	contents := []string{
		"branch (3) the keyless fallback reads the cache\nbranch 3 sees it",
		"Branch 3: the keyless fallback, then more",
		"see branch (12) the wrong one and branch (2) the right one.",
		"subbranch (3) the inner one\nbranch (3) the outer one",
		"## Keyless fallback (branch 3)\nbranch 3 sees",
		"- Keyless fallback path (Branch (3))\n",
		"1. Cold start (branch 3))\n",
		"Cold start (branch 3 )\nWarm start (branch 3)",
		"Cold start (branch 31)\nWarm start (branch 3)",
		"cold start (branch 3)\nWarm (branch 3)",
		"Cold start ( branch(3) )",
		"Cold start (step 3)\nWarm start (branch 3)",
		"BRANCH (3) THE upper case words here",
		"Kelvin (3) the kelvin sign folds to k",
		"kelvin 3 — the dash form of it",
		"kelvin 3 – the en dash form",
		"nothing here at all",
		"branch (3) the",
		"branch (3) the `code` span after two words",
		"x_branch (3) the joined word",
		"",
	}
	labels := [][2]string{{"branch", "3"}, {"Branch", "3"}, {"step", "3"}, {"kelvin", "3"}, {"Kelvin", "3"}, {"branch", "31"}, {"", "3"}}
	for _, content := range contents {
		for _, l := range labels {
			assert.Equal(t, resolveItemByRegexp(content, l[0], l[1]), resolveItem(content, l[0], l[1]),
				"content %q noun %q digits %q", content, l[0], l[1])
		}
	}
}
