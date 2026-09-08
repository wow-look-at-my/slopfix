package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every entry in english.xml carries a test attribute, and each has to fire. Without this an entry that stopped matching -- a typo, a phrase
// the boundary rule rejects, a rewrite shadowed by a drop -- would sit in the
// table looking enforced while doing nothing.
func TestEveryDropFires(t *testing.T) {
	require.NotEmpty(t, english.Drops)
	for _, d := range english.Drops {
		got := shortenFor(d.Test, surfaceOf(d.Where))
		assert.NotContains(t, strings.ToLower(got), strings.ToLower(d.Word),
			"<drop word=%q> did not fire on its own test %q, which gave %q", d.Word, d.Test, got)
		assert.NotEmpty(t, got, "a drop emptied the sentence")
	}
}

func TestEveryRewriteFires(t *testing.T) {
	require.NotEmpty(t, english.Rewrites)
	for _, r := range english.Rewrites {
		got := shortenFor(r.Test, surfaceOf(r.Where))
		assert.NotContains(t, strings.ToLower(got), strings.ToLower(r.From),
			"<rewrite from=%q> did not fire on %q, which gave %q", r.From, r.Test, got)
		if r.To != "" {
			assert.Contains(t, strings.ToLower(got), strings.ToLower(r.To),
				"<rewrite to=%q> is missing from %q", r.To, got)
		}
	}
}

func TestEveryPatternFires(t *testing.T) {
	require.NotEmpty(t, english.Patterns)
	for _, p := range english.Patterns {
		got := shortenFor(p.Test, surfaceOf(p.Where))
		assert.Equal(t, p.Expect, got,
			"<pattern match=%q> gave the wrong answer on its own test", p.Match)
	}
}

// A pattern carries the link SHAPE. It cannot carry linkrefs' guarantee, which
// is that a reference resolves before it becomes a link, so this pins that the
// these are different jobs rather than a replacement for each other.
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
		src := "package p\n\n// " + f.Test + "\nconst p = 1\n"
		got := Suggest("x.go", src)
		require.Len(t, got, 1, "<flag phrase=%q> did not fire on %q", f.Phrase, f.Test)
		assert.Equal(t, f.Phrase, got[0].Phrase)
		assert.Equal(t, f.Say, got[0].Say)
		assert.Equal(t, 3, got[0].Line)
	}
}

// A flag is advice, never a rewrite. Guessing a replacement would put a wrong
// sentence into a comment the reader trusts.
func TestAFlaggedPhraseIsNeverRewritten(t *testing.T) {
	src := "package p\n\n// See above for the reason.\nconst p = 1\n"
	out, changed := Fix("x.go", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
	assert.NotEmpty(t, Suggest("x.go", src), "but it is still reported")
}

// Length and prose are separate advice: a file can be clean by either and not
// the other, so Suggest reads comments Check has nothing to say about.
func TestSuggestFiresOnACommentThatFitsItsCode(t *testing.T) {
	src := "package p\n\n// A hack.\nconst p = 1\n"
	assert.Empty(t, Check("x.go", src), "short enough to pass the length rule")
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
