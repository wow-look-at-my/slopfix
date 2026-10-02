package commentfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/commentfix"
)

// Only the pragma syntax is a pragma. A line that opens with the word and then
// says something is prose the rule reads.
func TestProseAfterAPragmaWordIsStillProse(t *testing.T) {
	src := "# shellcheck is a linter we run, and this line explains at great length why the script below is written the way it is today.\n" +
		"run\n"
	assert.NotEmpty(t, commentfix.CheckLength("a.sh", src))
}

// The free text a pragma carries counts, so an essay cannot hide behind it.
// The cut takes the free text and keeps the pragma.
func TestAPragmaDescriptionCountsAndIsCut(t *testing.T) {
	src := "// eslint-disable-next-line no-console -- this description goes on and on about the history of the call below and why it was written, well past the code\n" +
		"console.log(x);\n"
	assert.NotEmpty(t, commentfix.CheckLength("a.ts", src))
	fixed, changed := commentfix.FixLength("a.ts", src)
	assert.True(t, changed)
	assert.Equal(t, "// eslint-disable-next-line no-console\nconsole.log(x);\n", fixed)
}

// A lint pragma between a doc block and its code is for the linter. The cut
// keeps it, and keeps the doc block closed.
func TestALintPragmaUnderADocBlockSurvivesTheCut(t *testing.T) {
	src := "/**\n" +
		" * A captured output stream that also carries a json helper, so every\n" +
		" * ordinary string operation still works on it, and then it says a great\n" +
		" * deal more about the runtime value than the short type below needs.\n" +
		" *\n" +
		" * The runtime value is a boxed String, so typeof reads object and a strict\n" +
		" * comparison against a string literal is false, which is a footgun.\n" +
		" */\n" +
		"// eslint-disable-next-line local/no-callable-primitive-intersection -- known\n" +
		"type OutputStream = string & { json<T = unknown>(): T };\n"
	fixed, changed := commentfix.FixLength("a.ts", src)
	assert.True(t, changed)
	assert.Contains(t, fixed, "// eslint-disable-next-line local/no-callable-primitive-intersection -- known\ntype OutputStream", fixed)
	assert.Equal(t, 1, strings.Count(fixed, "*/"), fixed)
	assert.Empty(t, commentfix.CheckLength("a.ts", fixed), fixed)
}
