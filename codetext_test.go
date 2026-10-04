package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
)

// testsSnippet is TypeScript held in a .txt file, the way a scratch copy of a
// replacement is kept. Read as prose, its lines were joined into one paragraph
// and its statements were capitalised.
const testsSnippet = `  it('a REFUSED force-merge is loud and inert: no wait, no refresh, no arm - the next contact retries', async () => {
    installMockFetch([
      tokenRoute(),
      { method: 'POST', test: (u) => u.endsWith('/hook/pr-describe'), status: 202, body: { run_id: 'run1' } },
    ]);
    await throttle();
    const cap = new CaptureLog();
    await handle('pull_request', prPayload('o/r', zdPr(), 'synchronize'), env, cap);
    assert.equal(callsTo('PUT', '/pulls/5/merge').length, 1, 'exactly one merge attempt per contact');
    assert.ok(cap.errs.some((m) => m.includes('refused')), ` + "`the refusal must be loud, got: ${cap.errs.join(' | ')}`" + `);
  });

  it('a BEHIND zero-diff head is force-merged as-is: the bypass needs no refresh first', async () => {
    await throttle();
    assert.equal(callsTo('PUT', '/update-branch').length, 0, 'no update-branch either');
  });
`

func TestCodeInATextFileIsNotProse(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{Path: "zd-tests.replacement.ts.txt", Content: testsSnippet})
	assert.Equal(t, testsSnippet, repair.Text, "a repair rewrote code as prose")
	assert.False(t, repair.Changed)
	assert.Empty(t, slopfix.CheckContent("zd-tests.replacement.ts.txt", testsSnippet), "a prose rule read code")
}

// A paragraph of code inside a document is code, though no fence marks it.
func TestAnUnfencedParagraphOfCodeIsNotProse(t *testing.T) {
	doc := "The steps below run the merge, and the\nlast one waits on it.\n\nawait throttle();\nconst cap = new CaptureLog();\n"
	repair := slopfix.Fix(slopfix.Request{Path: "notes.txt", Content: doc})
	assert.Equal(t, "The steps below run the merge, and the last one waits on it.\n\nawait throttle();\nconst cap = new CaptureLog();\n", repair.Text)
}
