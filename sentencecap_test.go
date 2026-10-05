package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// A comment that trips both the ratio check and the sentence cap. Both
// repairs settle in one fix, and a second fix changes nothing.
func TestTheRatioAndTheSentenceCapSettleTogether(t *testing.T) {
	src := "package demo\n\n" +
		"// The loader reads every cached manifest from the shared store and rebuilds the\n" +
		"// index of plugin hooks for each workspace the user opens in the editor during\n" +
		"// startup of the session. It holds a lock while it reads. It drops the lock\n" +
		"// before it writes the index, so a second editor never waits on the first.\n" +
		"func Load() {}\n"
	ids := findingIDs(slopfix.CheckContent("demo.go", src))
	require.Contains(t, ids, commentfix.IDLength)
	require.Contains(t, ids, ste.IDSentenceCap)

	once := slopfix.Fix(slopfix.Request{Content: src, Path: "demo.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	twice := slopfix.Fix(slopfix.Request{Content: once.Text, Path: "demo.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	assert.Equal(t, once.Text, twice.Text)
	for _, f := range slopfix.CheckContent("demo.go", once.Text) {
		assert.True(t, ste.ByHand(f.Fix) || f.Warning(), "%s is left:\n%s", f, once.Text)
	}
	assert.Contains(t, once.Text, "// The loader reads every cached manifest from the shared store.", once.Text)
}

// The sentence cap reaches a code comment as its own check: a short comment
// over a long function still reports a sentence past the cap.
func TestCheckReportsALongSentenceInAComment(t *testing.T) {
	src := "package demo\n\n" +
		"// The loader reads every cached manifest from the shared store of the plugin cache in the home directory of the user on each start of a session in the editor window.\n" +
		"func Load() {\n" + strings.Repeat("\tstep()\n", 60) + "}\n"
	var found []ste.Finding
	for _, f := range slopfix.CheckContent("demo.go", src) {
		if f.ID == ste.IDSentenceCap {
			found = append(found, f)
		}
	}
	require.Len(t, found, 1)
	assert.True(t, ste.ByHand(found[0].Fix))
	assert.Equal(t, src, slopfix.Fix(slopfix.Request{Content: src, Path: "demo.go"}).Text)
}
