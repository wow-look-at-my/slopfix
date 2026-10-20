package commentfix

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/fork-args-doc.rs
var forkArgsDoc string

//go:embed testdata/step-comment.rs
var stepComment string

// A step that opens on "2)" and wraps is prose, not a list, so the length repair still fixes it.
func TestALengthRepairFixesAWrappedNumberedStep(t *testing.T) {
	fixed, _ := FixLength("x.rs", stepComment)
	assert.Empty(t, CheckLength("x.rs", fixed), "the step comment is still too long:\n%s", fixed)
}

// A repair keeps each list item on a line of its own. Joined into the prose
// around it, the doc only looks shorter.
func TestALengthRepairKeepsListItems(t *testing.T) {
	fixed, _ := FixLength("x.rs", forkArgsDoc)
	for _, line := range strings.Split(fixed, "\n") {
		prose := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "///"))
		if strings.HasPrefix(prose, "- ") {
			continue
		}
		assert.NotContains(t, prose, " - `", "a list item was joined onto other prose: %q", line)
		assert.NotContains(t, prose, ". - ", "a list item was joined onto other prose: %q", line)
	}
}
