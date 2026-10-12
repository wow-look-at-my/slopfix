package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
)

// Each input is a real sentence that `slopfix fix` broke. The repair must keep a
// code span whole, keep every word, and never cluster symbols as nouns.
func TestClauseRepairsOnRealRolloutInput(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			"passive with a modal keeps the code span whole",
			"These constraints MUST be satisfied by the thinking injection logic in `rewrite/thinking.go`:",
			"The thinking injection logic in `rewrite/thinking.go` MUST satisfy these constraints:",
		},
		{
			"an arrow chain is not a noun cluster",
			"**Pipeline**: Input → Parser → Optimizer → Emitter → Output",
			"**Pipeline**: Input → Parser → Optimizer → Emitter → Output",
		},
		{
			"a plus and a slash are not nouns",
			"Cross-platform desktop searchbar (Spotlight-style UI, Everything-style speed) in Go + Wails v2 + vanilla TypeScript/Vite.",
			"Cross-platform desktop searchbar (Spotlight-style UI, Everything-style speed) in Go + Wails v2 + vanilla TypeScript/Vite.",
		},
		{
			"a plus between nouns keeps its nouns in place",
			"Vanilla TypeScript + Vite, no framework, with a vitest + jsdom suite that is a hard CI gate.",
			"Vanilla TypeScript + Vite, no framework, with a vitest + jsdom suite that is a hard CI gate.",
		},
	}
	keep := func(id string) bool {
		return id == ste.IDPassive || id == ste.IDNounCluster || id == ste.IDTense
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, ste.FixSelected(c.in, keep))
		})
	}
}
