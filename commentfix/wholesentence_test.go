package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zeroDiffIndicators is a comment that heads a run of declarations with no
// blank line between them. Weighed against the first const alone, its repair
// cut it to "... and the description opens." and dropped both sentences
// after it.
const zeroDiffIndicators = `const before = 1;

// The zero-diff indicators, written before the merge: the title gains ZERO_DIFF_TITLE_PREFIX and the
// description opens with ZERO_DIFF_BODY_NOTE. Each is added only when missing. A failed PATCH is loud
// and does not hold up the merge.
export const ZERO_DIFF_TITLE_PREFIX = '[zero diff]';
export const ZERO_DIFF_BODY_NOTE = '> **Zero diff:**';
async function stampZeroDiff(repo: string, pr: any, baseRef: string, headRef: string, token: string): Promise<void> {
  const title = String(pr.title ?? '');
  const body = String(pr.body ?? '');
  const titled = title.startsWith(ZERO_DIFF_TITLE_PREFIX);
  const noted = body.startsWith(ZERO_DIFF_BODY_NOTE);
  if (titled && noted) return;
  const note = ` + "`${ZERO_DIFF_BODY_NOTE} every change on ${headRef} is already on ${baseRef}`" + `;
  const newTitle = titled ? title : ` + "`${ZERO_DIFF_TITLE_PREFIX} ${title || headRef}`" + `;
  const newBody = noted ? body : body ? ` + "`${note}\\n\\n${body}`" + ` : note;
  await updatePullText(repo, pr.number, newTitle, newBody, token);
}
`

// A comment heads the code that runs on below it until a blank line, so it is
// weighed against all of that code.
func TestACommentIsWeighedAgainstTheCodeParagraphItHeads(t *testing.T) {
	assert.Empty(t, CheckLength("handlers.ts", zeroDiffIndicators))
	out, _ := FixLength("handlers.ts", zeroDiffIndicators)
	assert.Equal(t, zeroDiffIndicators, out)
}

// A blank line ends the paragraph a comment heads.
func TestABlankLineEndsTheParagraphAComments(t *testing.T) {
	src := strings.Replace(zeroDiffIndicators, "export const ZERO_DIFF_BODY_NOTE", "\nexport const ZERO_DIFF_BODY_NOTE", 1)
	require.NotEmpty(t, CheckLength("handlers.ts", src))
}

// The cut removes whole sentences from the end, and never cuts into the
// sentence it keeps.
func TestTheCutKeepsWholeSentences(t *testing.T) {
	src := strings.Replace(zeroDiffIndicators, "export const ZERO_DIFF_BODY_NOTE", "\nexport const ZERO_DIFF_BODY_NOTE", 1)
	out, changed := FixLength("handlers.ts", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("handlers.ts", out))
	assert.Contains(t, commentProse(out), "the title gains ZERO_DIFF_TITLE_PREFIX and the description opens with ZERO_DIFF_BODY_NOTE.", "the opening sentence is cut into:\n%s", out)
}

// One sentence is never an essay, whatever it documents.
func TestASingleSentenceIsNeverCut(t *testing.T) {
	src := "package p\n\n" +
		"// cacheKeyFor returns the stable composite lookup key built from the resolved descriptor\n" +
		"// set index plus binding slot plus array element offset plus sampler identity hash value.\n" +
		"func cacheKeyFor() int { return 0 }\n"
	assert.Empty(t, CheckLength("p.go", src))
	out, _ := FixLength("p.go", src)
	assert.Equal(t, src, out)
}
