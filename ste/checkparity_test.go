package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
)

// Fix repairs what Check reports, when a quotation decides how the clause parses.
func TestFixRepairsASpliceCheckReadsThroughAQuotation(t *testing.T) {
	text := `Auto order is bwrap -> seatbelt -> docker: the native backends are platform-exclusive, so this reads as "the native sandbox for this OS, else docker".`
	require.NotEmpty(t, findings(text, ste.IDCommaSplice), "the fixture must be a finding")
	out := ste.Fix(text)
	assert.Empty(t, findings(out, ste.IDCommaSplice), "the splice is left in:\n%s", out)
	assert.Contains(t, out, `"the native sandbox for this OS, else docker"`, "the quotation stays as written")
}

// Fix divides a sentence Check counts over the cap when an aside holds a link,
// and no division lands inside the aside.
func TestFixDividesASentenceWhoseAsideHoldsALink(t *testing.T) {
	text := "The tool reads an *existing* host file into the temp directory so a command can modify a copy of it, " +
		"and it uses `inputs.copy` or `shared.copy` (see [file-format.md](file-format.md#copy-fixtures-inputscopy-and-sharedcopy))."
	require.NotEmpty(t, findings(text, ste.IDSentenceCap), "the fixture must be a finding")
	out := ste.Fix(text)
	assert.Empty(t, findings(out, ste.IDSentenceCap), "the sentence stays over the cap:\n%s", out)
	assert.Contains(t, out, "(see [file-format.md](file-format.md#copy-fixtures-inputscopy-and-sharedcopy))", "no division lands inside the aside:\n%s", out)
	assert.Contains(t, out, " It uses `inputs.copy` or `shared.copy`", "the clause opens the rest as it is:\n%s", out)
}

// A "to" infinitive that opens a sentence is no sentence alone. The
// instruction goes first, and the infinitive follows it behind "Do this".
func TestAnInfinitiveOpenerFollowsItsMainClause(t *testing.T) {
	text := "To pull an existing host file into the temp directory so a command can modify a copy of it, " +
		"use the copy key or the shared copy key in the file for the run."
	got := findings(text, ste.IDSentenceCap)
	require.NotEmpty(t, got, "the fixture must be a finding")
	assert.False(t, ste.ByHand(got[0].Fix), got[0].Fix)
	assert.Equal(t, "Use the copy key or the shared copy key in the file for the run. "+
		"Do this to pull an existing host file into the temp directory so a command can modify a copy of it.", ste.Fix(text))
}

// An aside that holds a link target is a single word, as any aside is.
func TestAnAsideWithALinkCountsAsAWord(t *testing.T) {
	assert.Equal(t, 3, ste.WordCount("a (see [Leak gates](URL)) b"))
	assert.Equal(t, 3, ste.WordCount("a (see b) c"))
	assert.Equal(t, 3, ste.WordCount("a (b c"))
}

// findings answers what Check reports under id.
func findings(text, id string) []ste.Finding {
	var out []ste.Finding
	for _, f := range ste.Check(text, 1) {
		if f.ID == id {
			out = append(out, f)
		}
	}
	return out
}
