package slopfix_test

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// commitMessage is a commit message as an agent writes it. Its body holds a
// sentence that opens on a name from code, and its trailers close it.
//
//go:embed testdata/commit-message.yml
var commitMessage string

// A fix of a commit message leaves each trailer on its own line, because git
// reads them a line at a time.
func TestAFixKeepsEachTrailerOnItsOwnLine(t *testing.T) {
	out := slopfix.Fix(slopfix.Request{Content: commitMessage, Path: "msg.txt"}).Text
	assert.True(t, strings.HasSuffix(out, "\n\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>\n"+
		"Claude-Session: https://claude.ai/code/session_01AMfYXHb8wiqSB4Gnub92W5\n"), out)
}

// A sentence that opens on a lower-case name from code is a sentence of its
// own. Welded to the one before it, a division handed its verb the subject of
// that sentence: "Its reads keeps a darwin FIFO".
func TestASentenceOpeningOnANameFromCodeKeepsItsSubject(t *testing.T) {
	assert.Equal(t, []string{"so its reads never reach that EOF.", "newFile now asks F_GETFL."},
		ste.Sentences("so its reads never reach that EOF. newFile now asks F_GETFL."))

	out := slopfix.Fix(slopfix.Request{Content: commitMessage, Path: "msg.txt"}).Text
	assert.NotContains(t, out, "Its reads keeps", out)
	assert.Contains(t, out, "newFile now asks F_GETFL", out)
	assert.Empty(t, quoted(slopfix.CheckContent("msg.txt", out)), out)
}
