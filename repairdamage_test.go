package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// Lines of a real AGENTS.md that the repairs once made wrong. Each must come
// back from a whole fix as it went in.
var untouchedLines = map[string]string{
	"path":             "See docs/reveal-layer.md for principal resolution.\n",
	"symbol-and-names": "- `internal/ghclient/` — GitHub REST + GraphQL API client.\n",
	"symbol-in-a-run":  "- `internal/webhook/` — Webhook HTTP handler + event parsing, and the dispatcher.\n",
	"capitals":         "- **A LOST DELIVERY IS THE QUIETEST FAILURE HERE, AND GITHUB NEVER RETRIES ONE.** Miss a push.\n",
	"actor-past-dash":  "- **JSON text is produced by MARSHALLING — a JSON literal is a constant, or it is a bug.** Every other way.\n",
	"capitals-run":     "- **A BRANCH TIP IS THE ANSWER A LOST DELIVERY RUINS WORST.** A consumer cannot notice it.\n",
	"symbol-heading":   "- **Timeline wire format + response compression**: the chart reads a columnar payload.\n",
	"adjective-opener": "- `internal/freshness/` — Generic cache freshness framework with zero GitHub knowledge.\n",
}

func TestTheRepairsLeaveEachDamagedLineAlone(t *testing.T) {
	for name, line := range untouchedLines {
		t.Run(name, func(t *testing.T) {
			repair := slopfix.Fix(slopfix.Request{Content: line, Path: "AGENTS.md"})
			assert.Equal(t, line, repair.Text)
		})
	}
}

// The control for the path: the same word in prose still takes its approved word.
func TestTheDictionaryStillRewritesTheWordInProse(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{Content: "The reveal layer gates every read.\n", Path: "AGENTS.md"})
	assert.Equal(t, "The show layer gates every read.\n", repair.Text)
}

// An emphasis marker in front of a passive clause stays in front of the active one.
func TestThePassiveRepairKeepsTheOpeningMarker(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{Content: "**The file is read by the gate.** It stops.\n", Path: "AGENTS.md"})
	assert.Equal(t, "**The gate reads the file.** It stops.\n", repair.Text)
}

// The controls for the cluster: a run of common nouns still folds. A run
// with markup between its words is reported and left as written.
func TestTheClusterRepairFoldsOnlyPlainNouns(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{Content: "The gate cache lookup table stopped.\n", Path: "AGENTS.md"})
	assert.Equal(t, "The table of the gate cache lookup stopped.\n", repair.Text)

	marked := "The gate *cache lookup* table stopped.\n"
	repair = slopfix.Fix(slopfix.Request{Content: marked, Path: "AGENTS.md"})
	assert.Equal(t, marked, repair.Text)
	var hits []ste.Finding
	for _, f := range slopfix.Warnings(marked) {
		if f.ID == ste.IDNounCluster {
			hits = append(hits, f)
		}
	}
	if assert.Len(t, hits, 1) {
		assert.True(t, ste.ByHand(hits[0].Fix), hits[0].Fix)
	}
}
