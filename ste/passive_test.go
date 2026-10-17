package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The passive repair names any actor after "by" as the subject, and keeps
// each emphasis delimiter around the words it marked.
func TestThePassiveRepairKeepsActorsAndMarkup(t *testing.T) {
	for in, want := range map[string]string{
		"The task is started by users.":        "Users start the task.",
		"The task is **started** by the gate.": "The gate **starts** the task.",
		"The task is started by **the gate**.": "**The gate** starts the task.",
		"**The task** is started by the gate.": "The gate starts **the task**.",
		"The task is started by __the gate__.": "__The gate__ starts the task.",
	} {
		got := FixSelected(in, func(id string) bool { return id == IDPassive })
		assert.Equal(t, want, got, in)
	}
}
