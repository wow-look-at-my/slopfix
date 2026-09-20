package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every entry in rules/ carries a test attribute, and each has to fire. Without this an entry that stopped matching -- a typo, a phrase
// the boundary rule rejects, a rewrite shadowed by a drop -- would sit in the
// table looking enforced while doing nothing.
func TestEveryDropFires(t *testing.T) {
	require.NotEmpty(t, english.Drops)
	for _, d := range english.Drops {
		require.NotEmpty(t, d.Tests, "<drop id=%q> carries no test", d.ID)
		for _, c := range d.Tests {
			got := shortenFor(c.In, surfaceOf(d.Where))
			assert.NotContains(t, strings.ToLower(got), strings.ToLower(d.Word),
				"<drop id=%q> did not fire on %q, which gave %q", d.ID, c.In, got)
			assert.NotEmpty(t, got, "a drop emptied the sentence")
			if c.Out != "" {
				assert.Equal(t, c.Out, got, "<drop id=%q> gave the wrong answer", d.ID)
			}
		}
	}
}

func TestEveryRewriteFires(t *testing.T) {
	require.NotEmpty(t, english.Rewrites)
	for _, r := range english.Rewrites {
		require.NotEmpty(t, r.Tests, "<rewrite id=%q> carries no test", r.ID)
		for _, c := range r.Tests {
			got := shortenFor(c.In, surfaceOf(r.Where))
			assert.NotContains(t, strings.ToLower(got), strings.ToLower(r.From),
				"<rewrite id=%q> did not fire on %q, which gave %q", r.ID, c.In, got)
			if r.To != "" {
				assert.Contains(t, strings.ToLower(got), strings.ToLower(r.To),
					"<rewrite id=%q> lost its replacement in %q", r.ID, got)
			}
			if c.Out != "" {
				assert.Equal(t, c.Out, got, "<rewrite id=%q> gave the wrong answer", r.ID)
			}
		}
	}
}

func TestEveryPatternFires(t *testing.T) {
	require.NotEmpty(t, english.Patterns)
	for _, p := range english.Patterns {
		require.NotEmpty(t, p.Tests, "<pattern id=%q> carries no test", p.ID)
		for _, c := range p.Tests {
			got := shortenFor(c.In, surfaceOf(p.Where))
			if c.Out == "" {
				assert.NotEqual(t, c.In, got, "<pattern id=%q> did not fire on %q", p.ID, c.In)
				continue
			}
			assert.Equal(t, c.Out, got,
				"<pattern id=%q> gave the wrong answer on %q", p.ID, c.In)
		}
	}
}

// A pattern carries the link SHAPE. It cannot carry linkrefs' guarantee, which
// is that a reference resolves before it becomes a link, so this pins that the
func TestAPatternLinksAReferenceItCannotVerify(t *testing.T) {
	got := Deslop("owner/repo#99999 is open")
	assert.Contains(t, got, "https://github.com/owner/repo/issues/99999")
}

// A bare #N is left alone: the repository it belongs to is not in the text.
func TestABareIssueNumberIsNotLinked(t *testing.T) {
	const s = "See #42 for the reason"
	assert.Equal(t, s, Deslop(s))
}

// surfaceOf picks the surface an entry's own test must run on.
func surfaceOf(where string) string {
	if where == "message" {
		return "message"
	}
	return "comment"
}

// A message-only entry must not touch source. The em dash joining clauses
// is a chat habit; in a comment the prose rules already call it a comma splice,
// and rewriting it here would fight them.
func TestAMessageOnlyEntryLeavesCommentsAlone(t *testing.T) {
	const s = "Yes — and there is precedent"
	assert.Equal(t, s, shortenFor(s, "comment"), "comment surface leaves it")
	assert.Equal(t, "Yes, there is precedent", Deslop(s))
}

// An entry with no where= applies on both surfaces.
func TestAnUnmarkedEntryAppliesEverywhere(t *testing.T) {
	const s = "It basically fails"
	assert.Equal(t, "It fails", shortenFor(s, "comment"))
	assert.Equal(t, "It fails", Deslop(s))
}

func TestAppliesTo(t *testing.T) {
	assert.True(t, appliesTo("", "message"))
	assert.True(t, appliesTo("", "comment"))
	assert.True(t, appliesTo("both", "message"))
	assert.True(t, appliesTo("message", "message"))
	assert.False(t, appliesTo("message", "comment"))
	assert.False(t, appliesTo("comment", "message"))
}

// Deslop rewrites and never annotates: what comes back is the message, not the
// message plus a note about it.
func TestDeslopLeavesCleanProseByteIdentical(t *testing.T) {
	const s = "The port this listens on, chosen when the gate was added."
	assert.Equal(t, s, Deslop(s))
}

func TestEveryFlagFires(t *testing.T) {
	require.NotEmpty(t, english.Flags)
	for _, f := range english.Flags {
		require.NotEmpty(t, f.Tests, "<flag id=%q> carries no test", f.ID)
		for _, c := range f.Tests {
			src := "package p\n\n// " + c.In + "\nconst p = 1\n"
			got := Suggest("x.go", src)
			require.Len(t, got, 1, "<flag id=%q> did not fire on %q", f.ID, c.In)
			assert.Equal(t, f.Phrase, got[0].Phrase)
			assert.Equal(t, f.Say, got[0].Say)
			assert.Equal(t, 3, got[0].Line)
		}
	}
}

// A flag is advice, never a rewrite. Guessing a replacement would put a wrong
// sentence into a comment the reader trusts.
func TestAFlaggedPhraseIsNeverRewritten(t *testing.T) {
	src := "package p\n\n// See above for the reason.\nconst p = 1\n"
	out, changed := FixLength("x.go", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
	assert.NotEmpty(t, Suggest("x.go", src), "but it is still reported")
}

// Length and prose are separate advice: a file can be clean by either and not
// the other, so Suggest reads comments Check has nothing to say about.
func TestSuggestFiresOnACommentThatFitsItsCode(t *testing.T) {
	src := "package p\n\n// A hack.\nconst p = 1\n"
	assert.Empty(t, CheckLength("x.go", src), "short enough to pass the length rule")
	assert.NotEmpty(t, Suggest("x.go", src), "and still worth saying something about")
}

// The control that proves the flags can stay quiet.
func TestSuggestIsSilentOnCleanProse(t *testing.T) {
	src := "package p\n\n// The port this listens on.\nconst p = 1\n"
	assert.Empty(t, Suggest("x.go", src))
}

// A phrase inside a longer word is not the phrase.
func TestAFlagMatchesWholeWordsOnly(t *testing.T) {
	src := "package p\n\n// The hackney carriage rate.\nconst p = 1\n"
	assert.Empty(t, Suggest("x.go", src), "hack must not match hackney")
}

// A language the rule does not parse gets no suggestions rather than a guess.
func TestSuggestSkipsAnUnparsedLanguage(t *testing.T) {
	assert.Empty(t, Suggest("x.lua", "-- A hack.\nx = 1\n"))
}
