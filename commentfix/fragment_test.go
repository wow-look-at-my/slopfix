package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
)

// joinedProse joins the prose of every line comment in src.
func joinedProse(src string) string {
	var words []string
	for _, line := range strings.Split(src, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "//")
		if ok {
			words = append(words, strings.Fields(rest)...)
		}
	}
	return strings.Join(words, " ")
}

// A length cut keeps whole sentences of the source, or leaves the comment
// alone. It never ends a sentence at a word the author did not end it at.
func TestALengthCutNeverWritesAFragment(t *testing.T) {
	for _, src := range []string{
		"package p\n\nfunc f(mods []string) {\n\tfor _, m := range mods {\n\t\tif m != \"\" {\n" +
			"\t\t\t// An org module records a placeholder version in both go.mod\n" +
			"\t\t\t// and modules.txt, and resolves to a branch head, so the two\n" +
			"\t\t\t// are not compared.\n" +
			"\t\t\tcontinue\n\t\t}\n\t}\n}\n",
		"package p\n\nfunc f(m string) {\n" +
			"\t// An org module has no version of its own: the token on the require line\n" +
			"\t// is a placeholder, and the real version is the head of the branch the\n" +
			"\t// target follows.\n" +
			"\t_ = m\n}\n",
		"package p\n\n" +
			"// of the branch the target follows. replacementFrom is reached from the\n" +
			"// context-free mvs.Reqs interface, as rawGoModData is.\n" +
			"func replacementFrom() {}\n",
	} {
		out, _ := FixLength("x.go", src)
		before := joinedProse(src)
		for _, sentence := range ste.Sentences(joinedProse(out)) {
			assert.Contains(t, before, strings.TrimSpace(sentence), "the cut wrote a sentence the source never ended there:\n%s", out)
		}
	}
}

// A single sentence has no cut that reads, so the block stays as written and
// the finding goes to a person.
func TestASingleSentenceTooLongIsReportedNotCut(t *testing.T) {
	src := "package p\n\nfunc f(m string) {\n" +
		"\t// An org module has no version of its own: the token on the require line\n" +
		"\t// is a placeholder, and the real version is the head of the branch the\n" +
		"\t// target follows.\n" +
		"\t_ = m\n}\n"
	out, changed := FixLength("x.go", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
	hits := CheckLength("x.go", src)
	require.Len(t, hits, 1)
	assert.False(t, hits[0].Repairable)
}
