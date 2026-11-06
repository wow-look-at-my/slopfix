package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// This doc comment from grok-build crashed `slopfix check` with an index out of range in lineEndsThought.
var emptyNextLineComment = []string{
	"/// Build the default action definitions. `mouse_reporting_toggle_enabled`",
	"///",
	"/// `mouse_reporting_toggle_enabled` gates the opt-in `ToggleMouseCapture` shortcut (see below); pass `false` for the standard set.",
}

func TestTightenSurvivesAnEmptyCommentLine(t *testing.T) {
	assert.NotPanics(t, func() { Tighten(emptyNextLineComment) })
}

func TestAnEmptyNextLineEndsNoThought(t *testing.T) {
	assert.False(t, lineEndsThought("the byte at the checkpoint", ""))
	assert.False(t, lineEndsThought("the byte at the checkpoint", "   "))
	assert.True(t, lineEndsThought("the byte at the checkpoint", "This handles a list"))
}
