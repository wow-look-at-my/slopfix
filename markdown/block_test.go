package markdown_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/markdown"
)

func prose(doc string) []markdown.Block {
	var out []markdown.Block
	for _, b := range markdown.Split(doc) {
		if b.Kind == markdown.Prose {
			out = append(out, b)
		}
	}
	return out
}

func TestAParagraphIsProseAndAFenceIsNot(t *testing.T) {
	blocks := prose("# Title\n\nA line\nwrapped by hand.\n\n```sh\nnot prose\n```\n")
	require.Len(t, blocks, 1)
	assert.Equal(t, "A line wrapped by hand.", blocks[0].Text())
	assert.Equal(t, 3, blocks[0].Start)
}

func TestAQuotationIsLeftAsItsAuthorWroteIt(t *testing.T) {
	doc := "> a quoted line\n> wrapped by its author\n"
	assert.Empty(t, prose(doc))
	assert.Equal(t, doc, markdown.Format(doc))
}

func TestASetextHeadingIsALabel(t *testing.T) {
	assert.Empty(t, prose("A Heading\n=========\n"))
}

func TestAnHTMLBlockIsNotProse(t *testing.T) {
	assert.Empty(t, prose("<details>\n<summary>More</summary>\n</details>\n"))
}

func TestFrontMatterIsNotProse(t *testing.T) {
	blocks := prose("---\ntitle: a page\n---\n\nThe body.\n")
	require.Len(t, blocks, 1)
	assert.Equal(t, "The body.", blocks[0].Text())
}

func TestATableIsAGrid(t *testing.T) {
	assert.Empty(t, prose("a | b\n--|--\nc | d\n"))
}

func TestALazyContinuationLineStaysInItsParagraph(t *testing.T) {
	blocks := prose("A paragraph\n    that the author indented.\n")
	require.Len(t, blocks, 1)
	assert.Equal(t, "A paragraph that the author indented.", blocks[0].Text())
}

func TestAListItemKeepsItsMarker(t *testing.T) {
	blocks := prose("- an item\n  wrapped\n- the next\n")
	require.Len(t, blocks, 2)
	assert.Equal(t, "-", blocks[0].Marker)
	assert.Equal(t, "an item wrapped", blocks[0].Text())
	assert.Equal(t, "- an item wrapped\n- the next\n", markdown.Format("- an item\n  wrapped\n- the next\n"))
}

// commitTrailers is the trailer block every commit message here ends with.
const commitTrailers = "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>\n" +
	"Claude-Session: https://claude.ai/code/session_017LJqj9QeyPBBaQJcbngeEo\n"

// Git reads a trailer a line at a time, so each line is a block of its own
// and Format joins none of them.
func TestATrailerBlockKeepsALinePerTrailer(t *testing.T) {
	doc := "Join the paragraph\nwrapped by hand.\n\n" + commitTrailers
	blocks := prose(doc)
	require.Len(t, blocks, 3)
	assert.Equal(t, "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>", blocks[1].Text())
	assert.Equal(t, "Claude-Session: https://claude.ai/code/session_017LJqj9QeyPBBaQJcbngeEo", blocks[2].Text())
	assert.Equal(t, "Join the paragraph wrapped by hand.\n\n"+commitTrailers, markdown.Format(doc))
}

// A paragraph with a line that is no trailer is prose, and joins.
func TestAParagraphThatOnlyOpensWithATrailerJoins(t *testing.T) {
	doc := "Note: the cap holds\nfor every reader.\n"
	assert.Equal(t, "Note: the cap holds for every reader.\n", markdown.Format(doc))
}

func TestASecondParagraphInAListItemKeepsItsIndent(t *testing.T) {
	doc := "- an item\n\n  a second\n  paragraph\n"
	assert.Equal(t, "- an item\n\n  a second paragraph\n", markdown.Format(doc))
}
