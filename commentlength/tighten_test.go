package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tightening runs before cutting, so a padded comment keeps every thought it
// had and simply says them in fewer words.
func TestAPaddedCommentIsTightenedRatherThanCut(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// Note that this is basically just the port that the server actually",
		"// listens on, and it is obviously very important to really understand",
		"// that we make use of it in order to bind the socket at startup time.",
		"func listen() error {",
		"\tln, err := net.Listen(\"tcp\", addr)",
		"\tif err != nil {",
		"\t\treturn err",
		"\t}",
		"\treturn serve(ln)",
		"}",
	}, "\n")

	out, changed := Fix("x.go", src)
	require.True(t, changed)

	// The thought survives whole: the cut would have taken the binding clause.
	assert.Contains(t, out, "bind the socket")
	// The padding does not.
	for _, word := range []string{"basically", "obviously", "really", "Note that", "in order to", "make use of"} {
		assert.NotContains(t, out, word, "filler survived")
	}
	assert.Contains(t, out, "return serve(ln)", "the code is untouched")
	assert.Empty(t, Check("x.go", out), "the tightened comment fits")
}

func TestFillerIsOnlyDroppedAsAWholeWord(t *testing.T) {
	assert.Equal(t, "The adjustment", shorten("The adjustment"), "'just' inside 'adjustment' is not filler")
	assert.Equal(t, "A basic block", shorten("A basic block"), "'basically' does not match 'basic'")
	assert.Equal(t, "Injustice", shorten("Injustice"))
	assert.Equal(t, "The port", shorten("The very port"))
}

func TestAShorterPhrasingReplacesTheLongOne(t *testing.T) {
	assert.Equal(t, "Because it fails", shorten("Due to the fact that it fails"))
	assert.Equal(t, "If it fails", shorten("In the event that it fails"))
	assert.Equal(t, "It can retry", shorten("It has the ability to retry"))
}

// Reflow must not break a word that stops meaning anything when split.
func TestReflowNeverBreaksALongWord(t *testing.T) {
	url := "https://example.invalid/a/very/long/path/that/exceeds/the/wrap/width/by/itself"
	out := reflow("See "+url+" for the rest.", "", "//", wrapWidth)
	joined := strings.Join(out, "\n")
	assert.Contains(t, joined, url, "the URL survived intact")
}

// A paragraph break is structure, not prose, and it survives the reflow.
func TestReflowKeepsParagraphBreaks(t *testing.T) {
	text := []string{
		"// The point.",
		"//",
		"// The elaboration that follows it.",
	}
	out, changed := tighten(text)
	require.True(t, changed || len(out) == len(text))
	assert.Contains(t, strings.Join(out, "\n"), "//\n", "the blank marker is still there")
}

// A block whose lines disagree about the marker is left alone: rewriting it would
// change more than the prose.
func TestAMixedBlockIsNotTightened(t *testing.T) {
	_, changed := tighten([]string{"// prose", "code()"})
	assert.False(t, changed)
}

// The control: a comment with no filler and no long phrasing is untouched by
// the tightening pass, so a clean file stays byte-identical.
func TestACleanCommentIsNotRewritten(t *testing.T) {
	src := "package p\n\n// The port.\nconst p = 1\n"
	out, changed := Fix("x.go", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
}

// A doc comment opens on the identifier it documents, and that identifier is
// often unexported. Capitalising it names a symbol the package does not have,
// so the capital is restored only where a deletion removed the opening word.
func TestTheOpeningWordKeepsItsCase(t *testing.T) {
	assert.Equal(t, "arityReach is how far back it looks",
		shorten("arityReach is basically how far back it looks"))
	assert.Equal(t, "english is the parsed table",
		shorten("english is actually the parsed table"))
}

// The control: a deletion that removes the opening word does restore a capital,
// or the sentence starts in lower case for no reason.
func TestALeadingDeletionRestoresTheCapital(t *testing.T) {
	assert.Equal(t, "It fails", shorten("Obviously it fails"))
}
