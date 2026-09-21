package slopfix

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/xml-validator/validator"
)

// schemaLocation reads the schema a document names for itself.
var schemaLocation = regexp.MustCompile(`noNamespaceSchemaLocation="([^"]+)"`)

// borrowedTrees hold XML this repository did not write.
var borrowedTrees = set.Of(".claude", "testdata", "node_modules")

// TestEveryXMLNamesASchemaAndMeetsIt walks the repository, because a rule file
// that names no schema goes unchecked, and a schema the document has outgrown
// says a document is wrong when the schema is.
func TestEveryXMLNamesASchemaAndMeetsIt(t *testing.T) {
	found := 0
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if borrowedTrees.Contains(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".xml" {
			return nil
		}
		found++
		t.Run(path, func(t *testing.T) {
			text, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.True(t, strings.Contains(string(text), `version="1.1"`),
				"%s declares no XML 1.1 version", path)

			named := schemaLocation.FindStringSubmatch(string(text))
			require.NotNil(t, named, "%s names no schema to be checked against", path)

			schema := filepath.Join(filepath.Dir(path), named[1])
			assert.NoError(t, validator.ValidateWithSchemaFile(path, schema))
		})
		return nil
	})
	require.NoError(t, err)
	assert.NotZero(t, found, "the walk read no XML at all, so it asserted nothing")
}
