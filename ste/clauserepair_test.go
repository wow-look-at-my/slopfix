package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
)

// clauseRepair applies the clause warning repairs alone.
func clauseRepair(text string) string {
	return ste.FixSelected(text, func(id string) bool {
		return id == ste.IDPassive || id == ste.IDTense || id == ste.IDNounCluster
	})
}

// Each clause repair moves whole code spans and whole bold runs, writes the verb
// a participle belongs to, and keeps what leads the clause. That clause is in
// front of it.
func TestClauseRepairsKeepTheSentenceWhole(t *testing.T) {
	for name, c := range map[string]struct{ in, want string }{
		"a code span the actor names moves whole": {
			"The tripwires themselves are asserted by `.github/dats-fixtures/cache-profile.dats`, run by the dats action.",
			"`.github/dats-fixtures/cache-profile.dats` asserts the tripwires themselves, run by the dats action.",
		},
		"a code span the subject names moves whole": {
			"`src/vet` is untouched by this change.",
			"This change untouches `src/vet`.",
		},
		"an introductory phrase stays in front": {
			"As a result, the sites are deduplicated by `file:line` for one vet run.",
			"As a result, `file:line` for one vet run deduplicates the sites.",
		},
		"the actor ends at a parenthesis": {
			"The sites are deduplicated by the vet run (`resetMapSetWarnings`).",
			"The vet run deduplicates the sites (`resetMapSetWarnings`).",
		},
		"the actor ends at a dash": {
			"The chart is pinned by an equality test -- `TestRender` -- because the assertions pass.",
			"An equality test pins the chart -- `TestRender` -- because the assertions pass.",
		},
		"the actor ends at a subordinator": {
			"The file is read by the gate because the cache is cold.",
			"The gate reads the file because the cache is cold.",
		},
		"a clause after a coordinator keeps its opening": {
			"The plain variant holds no tests, so `deadcode` is answered by the richest variant.",
			"The plain variant holds no tests, so the richest variant answers `deadcode`.",
		},
		"a bold run around the clause stays around it": {
			"The plain variant holds no tests, so **`deadcode` is answered by the richest variant** (`richestVariants`).",
			"The plain variant holds no tests, so **the richest variant answers `deadcode`** (`richestVariants`).",
		},
		"a silent e comes back after dg": {
			"The text is judged by the gate.",
			"The gate judges the text.",
		},
		"a silent e comes back after a consonant and at": {
			"The sites are deduplicated by the run.",
			"The run deduplicates the sites.",
		},
		"a two-vowel stem takes no e": {
			"The file is treated by the gate.",
			"The gate treats the file.",
		},
		"a stem that ends on two consonants takes no e": {
			"The file is asserted by the gate.",
			"The gate asserts the file.",
		},
		"a one-syllable stem takes its e back": {
			"The file is named by the gate.",
			"The gate names the file.",
		},
		"a qu stem reads its u as a consonant": {
			"The file is quoted by the gate.",
			"The gate quotes the file.",
		},
		"a two-syllable stem takes no e": {
			"The file is opened by the gate.",
			"The gate opens the file.",
		},
		"vs joins two clusters and is not a noun": {
			"Fix mode vs check mode stays.",
			"Fix mode vs check mode stays.",
		},
	} {
		assert.Equal(t, c.want, clauseRepair(c.in), name)
	}
}

// A cluster that opens the sentence hands its capital to the head that now
// opens it.
func TestAClusterThatOpensTheSentenceMovesItsCapital(t *testing.T) {
	assert.Regexp(t, `^Guard of the [Bb]uild-log duration regression stops\.$`, clauseRepair("Build-log duration regression guard stops."))
}
