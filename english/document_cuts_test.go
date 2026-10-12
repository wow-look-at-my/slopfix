package english

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A link label names its target. A cut there leaves a link with no text.
func TestAPatternNeverCutsALinkLabel(t *testing.T) {
	in := "Divergences are tracked in [issue #9](https://example.com/issues/9) and stay open."
	out, _ := FixN(in, Document)
	assert.Contains(t, out, "[issue #9](https://example.com/issues/9)")
}

// The shrug refuses to explain. An instruction not to ask the user something is
// a rule the text states.
func TestAnInstructionNotToAskSurvives(t *testing.T) {
	in := `A defect is not a menu item. Do not ask the user "want me to fix this?"; fix it.`
	out, _ := FixN(in, Document)
	assert.Contains(t, out, "Do not ask the user")
	assert.Equal(t, "The cap holds.", Fix("The cap holds. Do not ask why.", Document))
}
