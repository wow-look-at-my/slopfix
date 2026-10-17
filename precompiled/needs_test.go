package precompiled_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/wow-look-at-my/slopfix/cmd"
	"github.com/wow-look-at-my/slopfix/precompiled"
)

func TestRequiredNamesTheLiteralsEveryMatchHolds(t *testing.T) {
	cases := map[string][]string{
		`(?i)\s*\b(?:previously|formerly)\b[,]?`: {"FORMERLY", "PREVIOUSLY"},
		`(?i)\s*\bfor now\b[,]?`:                  {"FOR NOW"},
		`(?i)\bthe (?:old|previous|legacy) (\S+)`: {"THE LEGACY ", "THE OLD ", "THE PREVIOUS "},
		`abc`:                                     {"ABC"},
		`a+b`:                                     {"A"},
		`x*`:                                      nil,
		`\d+`:                                     {"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"},
		`(?:foo|.+)`:                              nil,
		`colou?r`:                                 {"COLOR", "COLOUR"},
	}
	for pattern, want := range cases {
		assert.Equal(t, want, precompiled.Required(pattern), pattern)
	}
}

func TestFoldMatchesWhatCaseFoldingMatches(t *testing.T) {
	assert.Equal(t, precompiled.Fold("kelvin"), precompiled.Fold("Kelvin"))
	assert.Equal(t, precompiled.Fold("STRASSE"), precompiled.Fold("ſtrasse"))
	assert.Equal(t, "ABC-1", precompiled.Fold("aBc-1"))
}

// corpus reads text from the repository, and variants a case-folding pattern
// matches in other case.
func corpus(t *testing.T) []string {
	var out []string
	for _, glob := range []string{"../english/*.xml", "../rules/*.xml", "../docs/*.md", "../*.md"} {
		paths, err := filepath.Glob(glob)
		require.NoError(t, err)
		for _, path := range paths {
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			for line := range strings.Lines(string(raw)) {
				out = append(out, line, strings.ToUpper(line), strings.ReplaceAll(strings.ToLower(line), "k", "K"))
			}
		}
	}
	return out
}

// A skipped regexp that would have matched loses a finding, so the literals
// must hold in every text a pattern matches.
func TestEveryMatchHoldsALiteralItNeeds(t *testing.T) {
	texts := corpus(t)
	require.NotEmpty(t, texts)
	for _, pattern := range precompiled.Requested() {
		re := regexp.MustCompile(pattern)
		needs := precompiled.Needs(re)
		for _, text := range texts {
			if re.MatchString(text) {
				require.True(t, precompiled.HoldsAny(precompiled.Fold(text), needs),
					"pattern %q matches %q, which holds none of %q", pattern, text, needs)
			}
		}
	}
}

// go generate writes every pattern the program asks for. A miss compiles at
// run time, so a stale generated file fails here.
func TestTheProgramCompilesNoPatternAtRunTime(t *testing.T) {
	require.NotEmpty(t, precompiled.Requested())
	assert.Empty(t, precompiled.Missed(), "run go generate ./precompiled")
}
