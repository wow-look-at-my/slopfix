package english

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// surfaceOf picks the surface an entry's own test must run on.
func surfaceOf(where string) string {
	if where == "message" {
		return "message"
	}
	return "comment"
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

// A doc comment opens on the identifier it documents, and that identifier is
// often unexported. Capitalising it names a symbol the package does not have,
// so the capital is restored only where a deletion removed the opening word.
func TestTheOpeningWordKeepsItsCase(t *testing.T) {
	assert.Equal(t, "arityReach is how far back it looks",
		Fix("arityReach is basically how far back it looks", Comment))
	assert.Equal(t, "english is the parsed table",
		Fix("english is actually the parsed table", Comment))
}

// The control: a deletion that removes the opening word does restore a capital,
// or the sentence starts in lower case for no reason.
func TestALeadingDeletionRestoresTheCapital(t *testing.T) {
	assert.Equal(t, "It fails", Fix("Obviously it fails", Comment))
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
