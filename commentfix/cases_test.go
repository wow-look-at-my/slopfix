package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What the table writes for a whole line is stated in rules/numbers-cases.xml,
// where somebody adding a case edits no Go. An entry's own test holds that
// entry. These hold the rewrites, the rephrasings and the patterns together,
// in the order the repair applies them.
func TestEveryWholeLineTestHolds(t *testing.T) {
	require.NotEmpty(t, numbersTable.Tests)

	for _, c := range numbersTable.Tests {
		t.Run(c.In, func(t *testing.T) {
			got := Reword(c.In)
			if c.Out == "" {
				assert.NotEqual(t, c.In, got, "the table left the line alone")
				return
			}
			assert.Equal(t, c.Out, got)
		})
	}
}
