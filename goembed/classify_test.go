package goembed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each file under testdata/classify names its expected extension. A file that
// ends in .none must read as no foreign language.
func TestClassifyReadsEachFixtureAsItsExtension(t *testing.T) {
	entries, err := os.ReadDir("testdata/classify")
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			text, err := os.ReadFile(filepath.Join("testdata/classify", e.Name()))
			require.NoError(t, err)
			lang, ok := Classify(string(text))
			if strings.HasSuffix(e.Name(), ".none") {
				assert.False(t, ok, "%s reads as %s", e.Name(), lang.Name)
				return
			}
			require.True(t, ok, "%s reads as no language", e.Name())
			assert.Equal(t, filepath.Ext(e.Name()), lang.Ext)
		})
	}
}
