package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// A /* */ block laid out under its opener with no star on each line. The length
// cut leaves the count on a line of its own, and the number repair takes it out.
const alignedBlock = "const reveal = (() => {\n" +
	"  /* Walks light DOM and open shadow roots alike, because the composer\n" +
	"     builds its field and button inside its own shadow root - a document-only\n" +
	"     query would light the standalone controls and miss those two. An element\n" +
	"     whose definition has not loaded yet has no shadow root to descend into, so\n" +
	"     it gets revisited once the definition lands. */\n" +
	"  function visit(el: Element): void {\n" +
	"    if (el.matches(SELECTOR)) track(el as HTMLElement);\n" +
	"    if (el.shadowRoot) descend(el.shadowRoot);\n" +
	"    else if (el.localName.includes('-') && !customElements.get(el.localName)) {\n" +
	"      customElements.whenDefined(el.localName)\n" +
	"        .then(() => { if (el.isConnected) visit(el); });\n" +
	"    }\n" +
	"  }\n" +
	"  return { visit };\n" +
	"})();\n"

// The count goes and the sentence still ends on its words, with the period on
// the line it closes.
func TestACountCutFromAnAlignedBlockLeavesNoLonePeriod(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{Content: alignedBlock, Path: "reveal.ts", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	for _, line := range strings.Split(repair.Text, "\n") {
		trimmed := strings.TrimSpace(line)
		assert.False(t, strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, ","), "a line opens on a mark:\n%s", repair.Text)
	}
	assert.NotContains(t, repair.Text, "two", repair.Text)
	assert.Contains(t, repair.Text, "miss those.", "the sentence ends on its words:\n%s", repair.Text)
	assert.Equal(t, 1, strings.Count(repair.Text, "*/"), repair.Text)
	assert.Contains(t, repair.Text, "  function visit(el: Element): void {\n", "the code is untouched")
	assert.Empty(t, commentfix.Check("reveal.ts", repair.Text), repair.Text)
}
