package table_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/table"
)

// A digits class claims a number however it is spelled, and nothing else.
func TestDigitsClassClaimsANumberInDigits(t *testing.T) {
	lex := table.NewLexicon([]table.Class{
		{Name: "number", Digits: true, Words: []string{"fifteen"}},
		{Name: "noun", Open: true, Except: []string{"number"}},
	})
	assert.True(t, lex.Is("15", "number"))
	assert.True(t, lex.Is("fifteen", "number"))
	assert.False(t, lex.Is("15s", "number"))
	assert.False(t, lex.Is("", "number"))
	assert.False(t, lex.Is("15", "noun"), "a number in digits is not a noun")
	assert.True(t, lex.Is("minute", "noun"))
}
