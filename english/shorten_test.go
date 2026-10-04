package english

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func surfaceOf(where string) string {
	if where == "message" {
		return "message"
	}
	return "comment"
}

// A cut at the end of a clause takes the comma before it too. Otherwise the
// comma sits against the closing punctuation.
func TestACutLeavesNoCommaAgainstTheStop(t *testing.T) {
	assert.Equal(t, "It uses the blessed *endpoint*.", Fix("It uses the blessed *endpoint*, for now.", Document))
	assert.Equal(t, "It retries once; then it stops.", Fix("It retries once, for now; then it stops.", Document))
	assert.Equal(t, "Run it (once).", Fix("Run it (once, for now).", Document))
	assert.Equal(t, "Keep 1,.5 as written.", Fix("Keep 1,.5 as written.", Document), "a comma inside a token is not a stranded one")
}

// Every entry declares its own worked examples, and each has to fire. Without
// this an entry that stopped matching -- a typo, a phrase the boundary rule
// rejects, a rewrite shadowed by a drop -- would sit in the table looking
// enforced while doing nothing.
func TestEveryDropFires(t *testing.T) {
	require.NotEmpty(t, Drops())
	for _, d := range Drops() {
		for _, c := range d.Tests() {
			got := Fix(c.In, surfaceOf(d.Where))
			assert.NotContains(t, strings.ToLower(got), strings.ToLower(d.Word),
				"<drop word=%q> did not fire on %q, which gave %q", d.Word, c.In, got)
			assert.NotEmpty(t, got, "a drop emptied the sentence")
		}
	}
}

func TestEveryRewriteFires(t *testing.T) {
	require.NotEmpty(t, Rewrites())
	for _, r := range Rewrites() {
		for _, c := range r.Tests() {
			got := Fix(c.In, surfaceOf(r.Where))
			assert.NotContains(t, strings.ToLower(got), strings.ToLower(r.From),
				"<rewrite from=%q> did not fire on %q, which gave %q", r.From, c.In, got)
			if r.To != "" {
				assert.Contains(t, strings.ToLower(got), strings.ToLower(r.To),
					"<rewrite to=%q> is missing from %q", r.To, got)
			}
		}
	}
}

func TestEveryPatternFires(t *testing.T) {
	require.NotEmpty(t, Patterns())
	for _, p := range Patterns() {
		for _, c := range p.Tests() {
			got := Fix(c.In, surfaceOf(p.Where))
			assert.Equal(t, c.Out, got,
				"<pattern match=%q> gave the wrong answer on %q", p.Match, c.In)
		}
	}
}

func TestEveryShapeFires(t *testing.T) {
	require.NotEmpty(t, Shapes())
	for _, s := range Shapes() {
		for _, c := range s.Tests() {
			assert.Equal(t, c.Out, Fix(c.In, surfaceOf(s.Where)),
				"<shape id=%q> gave the wrong answer on %q", s.ID, c.In)
		}
	}
}

// A <test> under the table's root belongs to no entry: it states what the
// repair writes for a whole line, which is where prose reaching several
// entries, or reaching none, is said.
func TestEveryWholeLineCaseHolds(t *testing.T) {
	require.NotEmpty(t, Cases())
	for _, c := range Cases() {
		t.Run(c.In, func(t *testing.T) {
			assert.Equal(t, c.Out, Fix(c.In, Comment))
		})
	}
}

// A flag names a phrase and never rewrites it, so its own test must still be
// found and the prose must come back untouched.
func TestEveryFlagFires(t *testing.T) {
	require.NotEmpty(t, Flags())
	for _, f := range Flags() {
		for _, c := range f.Tests() {
			assert.NotEmpty(t, Flagged(c.In),
				"<flag phrase=%q> did not fire on %q", f.Phrase, c.In)
			assert.Equal(t, c.In, Fix(c.In, "comment"),
				"<flag phrase=%q> rewrote its own test", f.Phrase)
		}
	}
}

// A pattern replacement keeps the capital of the sentence it rewrites, in the middle of a paragraph as at its start.
func TestAPatternKeepsTheCapitalOfAMidParagraphSentence(t *testing.T) {
	got := Fix("A replacement that fails is removed. The previous image comes back.", Document)
	assert.Equal(t, "A replacement that fails is removed. The image comes back.", got)
}

// A hyphenated compound is one word. A cut of its first half leaves a fragment such as "the-recorded".
func TestAPatternNeverCutsHalfOfAHyphenatedWord(t *testing.T) {
	in := "oldBase is the previously-recorded base identity for reporting."
	assert.Equal(t, in, Fix(in, Comment))
}

// A quotation names a phrase. A cut inside it leaves an unbalanced quote mark.
func TestAPatternLeavesAQuotationWhole(t *testing.T) {
	in := `Comments state the invariant. No changelogs, no dates, no "this used to".`
	assert.Equal(t, in, Fix(in, Document))
}

// A code span is a command. A filler-word drop inside it breaks the command.
func TestAPatternLeavesACodeSpanWhole(t *testing.T) {
	in := "Compiled by `just build` (tsc) to the asset."
	assert.Equal(t, in, Fix(in, Comment))
	// Fix joins whitespace, so a wrapped span comes back on one line with its words whole.
	assert.Equal(t, in, Fix("Compiled by `just\nbuild` (tsc) to the asset.", Comment))
}

// A cut to the end of a sentence must not leave it on a word that opens what the cut took.
func TestAPatternCutNeverEndsASentenceOnAnOpener(t *testing.T) {
	in := "An operator sees a consumer that stopped being told rather than that silently never was."
	assert.Equal(t, in, Fix(in, Comment))
	assert.Equal(t, "The cap holds.", Fix("The cap holds stopped being read at start.", Comment))
}
