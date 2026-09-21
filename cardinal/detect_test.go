package cardinal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// substrates names each reader the way a rules file spells it.
var substrates = map[string]Substrate{
	"prose":   Prose,
	"comment": Comment,
	"gate":    Gate,
}

// What each substrate reports for a line is stated in rules/numbers-detect.xml,
// where somebody adding a case edits no Go. A case with no <found> says the
// substrate reports nothing, which is how an exemption is written down.
func TestEveryDetectionCaseHolds(t *testing.T) {
	require.NotEmpty(t, numbersTable.Detects)

	for _, c := range numbersTable.Detects {
		name := c.Substrate + ": " + c.In
		t.Run(name, func(t *testing.T) {
			s, known := substrates[c.Substrate]
			require.True(t, known, "no substrate is named %q", c.Substrate)
			if len(c.Found) == 0 {
				assert.Empty(t, texts(c.In, s), c.Why)
				return
			}
			assert.Equal(t, c.Found, texts(c.In, s), c.Why)
		})
	}
}
