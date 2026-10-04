package slopfix_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	jsonvalidator "github.com/wow-look-at-my/json-validator/validator"
	"github.com/wow-look-at-my/slopfix/forkscope"
)

// forklist.schema.json is the shape the org's .github repository checks its
// fork list against. forkscope reads the list, and refuses exactly the lists
// the schema refuses.
func TestTheForkListReaderAgreesWithTheSchema(t *testing.T) {
	schema, err := os.ReadFile("forklist.schema.json")
	require.NoError(t, err)
	v, err := jsonvalidator.NewFromBytes("forklist.schema.json", schema, jsonvalidator.Options{})
	require.NoError(t, err)

	for name, list := range map[string]string{
		"a fork":                   `{"o/fork": "https://example.invalid/up"}`,
		"comments":                 "// forks\n{\"o/fork\": \"https://example.invalid/up\", /* x */}\n",
		"no forks":                 `{}`,
		"an entry without a URL":   `{"o/fork": ""}`,
		"a name with no owner":     `{"fork": "https://example.invalid/up"}`,
		"a name with a space":      `{"o/my fork": "https://example.invalid/up"}`,
		"a name with three parts":  `{"o/fork/x": "https://example.invalid/up"}`,
		"a URL that is no string":  `{"o/fork": 1}`,
		"a list that is no object": `["o/fork"]`,
		"a null list":              `null`,
	} {
		_, readErr := forkscope.UpstreamFor(list, "o/fork", name)
		schemaErr := v.ValidateBytes([]byte(list), name).AsError()
		assert.Equal(t, schemaErr == nil, readErr == nil, "%s: the schema says %v, the reader says %v", name, schemaErr, readErr)
	}
}
