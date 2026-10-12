package syntax_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// tagOf answers the tag the parse gives the word written as w.
func tagOf(t *testing.T, text, w string) string {
	t.Helper()
	s := parse(t, text)
	for _, word := range s.Words {
		if word.Text == w {
			return word.Tag
		}
	}
	t.Fatalf("%q holds no word %q", text, w)
	return ""
}

// A -tion, -ment or -ness plural after a subject is the subject's verb.
func TestAnSFormAfterASubjectStaysAVerb(t *testing.T) {
	for text, w := range map[string]string{
		"The gate implements the check.":            "implements",
		"The page documents every flag.":            "documents",
		"The reviewer comments on each change.":     "comments",
		"The log mentions the failed job.":          "mentions",
		"The auditor questions every grant.":        "questions",
		"The cache functions as a second store.":    "functions",
		"Each parser implements the same contract.": "implements",
	} {
		assert.True(t, strings.HasPrefix(tagOf(t, text, w), "VB"), "%s: %q", text, w)
	}
}

// A determiner, an adjective or a coordination of names before the plural
// proves it a noun.
func TestAnSFormAfterANounOpenerIsAPlural(t *testing.T) {
	for text, w := range map[string]string{
		"Linux tests the C and Python implementations first.": "implementations",
		"The documents hold the flags.":                       "documents",
		"Two mentions name the job.":                          "mentions",
		"macOS builds the C and Python ones.":                 "ones",
	} {
		assert.True(t, strings.HasPrefix(tagOf(t, text, w), "NN"), "%s: %q", text, w)
	}
}
