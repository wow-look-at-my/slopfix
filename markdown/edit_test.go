package markdown_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/markdown"
)

const doc = "# Title\n\nThe gate reads\nevery file.\n\n```sh\nslopfix check .\n```\n"

// replace answers the edit that swaps the first old in doc for text.
func replace(old, text string) edit.Edit {
	at := strings.Index(doc, old)
	return edit.Edit{Start: at, End: at + len(old), Text: text}
}

func TestAProseRewriteLands(t *testing.T) {
	res := markdown.Apply(doc, []edit.Edit{replace("The gate reads\nevery file.", "The gate reads every file.")}, edit.Scope{})
	assert.Empty(t, res.Refused)
	assert.Contains(t, res.Text, "The gate reads every file.\n\n```sh")
}

// Each rewrite below would change a block the parser keeps verbatim.
func TestAProseRewriteCannotReachAVerbatimBlock(t *testing.T) {
	cases := map[string]edit.Edit{
		"an edit over a fence line":     replace("slopfix check .", "slopfix fix ."),
		"an edit over a heading":        replace("# Title", "# Other"),
		"a rewrite that opens a fence":  replace("every file.", "every file.\n```"),
		"a rewrite that adds a heading": replace("every file.", "every file.\n\n# New"),
	}
	for name, e := range cases {
		t.Run(name, func(t *testing.T) {
			res := markdown.Apply(doc, []edit.Edit{e}, edit.Scope{})
			require.Len(t, res.Refused, 1)
			assert.Equal(t, doc, res.Text)
		})
	}
}
