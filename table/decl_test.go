package table_test

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
)

// encoding/xml refuses a 1.1 declaration outright, and every document in this
// repository states 1.1. A loader that hands it the bytes as they sit on disk
// panics at init, which is how a whole build died.
func TestTheDeclaredVersionReachesTheParser(t *testing.T) {
	raw := []byte(`<?xml version="1.1" encoding="UTF-8"?>` + "\n<rules for=\"numbers\"/>\n")

	var doc struct {
		For string `xml:"for,attr"`
	}
	require.Error(t, xml.Unmarshal(raw, &doc), "the control: 1.1 is refused")
	require.NoError(t, xml.Unmarshal(table.Readable(raw), &doc))
	assert.Equal(t, "numbers", doc.For)
}

// Only the declaration is touched, and only when it states 1.1.
func TestNothingElseIsRewritten(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>` + "\n<rules for=\"x\">1.1</rules>\n")
	assert.Equal(t, raw, table.Readable(raw))

	body := []byte(`<?xml version="1.1"?>` + "\n<rules for=\"x\">keep 1.1 here</rules>\n")
	assert.Contains(t, string(table.Readable(body)), "keep 1.1 here")
}

// Every rules file loads. A file the folder carries that no loader can read is
// a rule that silently never fires.
func TestEveryRulesTargetLoads(t *testing.T) {
	for _, target := range []string{"numbers", "ste", "english", "comment-tails"} {
		got, err := table.Load(rules.FS, target)
		require.NoError(t, err, "target %s", target)
		assert.NotNil(t, got)
	}
}
