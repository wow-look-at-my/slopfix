package cardinal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/cardinal"
)

// Each line holds a number that is a value or a name, taken from a document
// the stale-count rule rewrote into nonsense.
var notCounts = []string{
	// An HTTP status code is a value the protocol answers with.
	"The row insert, the 409 conflict semantics and the 201 responses are shared with a full upload.",
	"Any status other than 201 or 409 there triggers a full upload.",
	"The match is deliberate. A 401 or 403 confirms that the module exists.",
	"Every probe outcome other than 200 and 404 logs at WARN.",
	"It answers HTTP 404 pages for every miss.",
	// A label names one item, and a leading zero is a label's spelling.
	"Migration 015 defines it: artifact_id, fmt, the client.",
	"Migration 014 pins that id on the project.",
	"They honor the Section 4 pins.",
	// A size ahead of the noun it describes.
	"It sends one create and four 1 MiB PATCH requests.",
	"It retries with 30 second timeouts.",
}

func TestAValueOrALabelIsNotAStaleCount(t *testing.T) {
	for _, prose := range notCounts {
		assert.Empty(t, cardinal.Find(prose, cardinal.Gate), "gate: %s", prose)
		assert.Empty(t, cardinal.Find(prose, cardinal.Prose), "prose: %s", prose)
	}
}

// The control: the same shapes with a number that does count a set.
func TestATallyBesideAValueIsStillACount(t *testing.T) {
	for _, prose := range []string{
		"That took 44s for 238 releases.",
		"It answers 404 for 12 projects.",
		"It ships 200 plugins.",
		"It sends four PATCH requests.",
	} {
		assert.NotEmpty(t, cardinal.Find(prose, cardinal.Gate), "gate: %s", prose)
	}
}
