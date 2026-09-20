package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/table"
)

// Every entry in the table drives its own case. An entry that has stopped
// firing says so here rather than sitting in the file looking enforced.
func TestEveryTableEntryFires(t *testing.T) {
	require.NotEmpty(t, numbersTable.Rewrites)
	require.NotEmpty(t, numbersTable.Patterns)

	require.NotEmpty(t, numbersTable.Rephrasings)
	for _, r := range numbersTable.Rewrites {
		fires(t, "rewrite", r.ID, r.Tests)
	}
	for _, p := range numbersTable.Patterns {
		fires(t, "pattern", p.ID, p.Tests)
	}
	for _, e := range numbersTable.Rephrasings {
		fires(t, "rephrase", e.ID, e.Tests)
	}
}

// fires holds an entry to every case it carries. A case naming no output only
// holds the entry to changing the prose at all.
func fires(t *testing.T, kind, id string, cases []table.Test) {
	t.Helper()
	require.NotEmpty(t, cases, "<%s id=%q> carries no test", kind, id)
	for _, c := range cases {
		got := Reword(c.In)
		if c.Out == "" {
			assert.NotEqual(t, c.In, got, "<%s id=%q> did not fire on %q", kind, id, c.In)
			continue
		}
		assert.Equal(t, c.Out, got, "<%s id=%q> did not fire on %q", kind, id, c.In)
	}
}

// Every class a match names is declared. A typo in a class name matches
// nothing and costs no error, so the entry silently stops covering the prose
// it was written for.
func TestEveryClassAMatchNamesIsDeclared(t *testing.T) {
	require.NotEmpty(t, numbersTable.Classes)
	declared := set.Of[string]("open")
	for _, c := range numbersTable.Classes {
		declared.Add(c.Name)
	}
	for _, e := range numbersTable.Rephrasings {
		for _, class := range e.Terms.Classes() {
			assert.True(t, declared.Contains(class), "match %q names the undeclared class %q", e.Match, class)
		}
	}
}

// The rule bans the cardinal whatever the table can repair. An entry removed
// or narrowed here changes what the repair WRITES. It must never change what
// the rule REPORTS, or a shape nothing covers goes quietly unreported.
func TestTheCardinalStaysBannedWhereTheTableRepairsNothing(t *testing.T) {
	for _, prose := range []string{
		"The count is one",
		"It holds one",
		"It takes the wrong one",
		"It owns the one below",
	} {
		assert.Equal(t, prose, Reword(prose), "the table repaired prose this case needs it to leave")
		assert.NotEmpty(t, cardinal.Find(prose, cardinal.Comment), "the rule stopped reporting %q", prose)
	}
}

// What the table says leaves no number behind. An entry whose replacement
// carried another number would send the repair straight to a cut.
func TestWhatTheTableSaysCarriesNoNumber(t *testing.T) {
	leaves := func(kind, id string, cases []table.Test) {
		for _, c := range cases {
			assert.Empty(t, cardinal.Find(Reword(c.In), cardinal.Comment),
				"<%s id=%q> leaves a number", kind, id)
		}
	}
	for _, r := range numbersTable.Rewrites {
		leaves("rewrite", r.ID, r.Tests)
	}
	for _, p := range numbersTable.Patterns {
		leaves("pattern", p.ID, p.Tests)
	}
	for _, e := range numbersTable.Rephrasings {
		leaves("rephrase", e.ID, e.Tests)
	}
}

<<<<<<< HEAD
// What the table writes for a whole line is stated in rules/numbers-cases.xml,
// where somebody adding a case edits no Go.
func TestEveryCaseHolds(t *testing.T) {
	require.NotEmpty(t, numbersTable.Cases)

	for _, c := range numbersTable.Cases {
		t.Run(c.Test, func(t *testing.T) {
			assert.Equal(t, c.Expect, Say(c.Test))
		})
	}
}
=======
// A qualified name is a single word, so no marker inside it opens a rewrite.
// The rule reads its own package's comments, and sync.a single time is what it found there.
func TestAQualifiedNameIsLeftWhole(t *testing.T) {
	for _, prose := range []string{
		"sync.Once guards it",
		"net/http serves it",
		"it calls Do.Once here",
	} {
		assert.Equal(t, prose, Reword(prose))
	}
	// The control: the same word standing alone still rewrites.
	assert.Equal(t, "the a single time flag", Reword("the once flag"))
}

func TestProseTheTableDoesNotCoverIsUntouched(t *testing.T) {
	assert.Equal(t, "It reserves a slot and publishes it", Reword("It reserves a slot and publishes it"))
}

// A comment line often continues a wrapped sentence rather than opening it, so
// the repair keeps the opening case the author wrote.
func TestTheOpeningCaseIsTheOneTheAuthorWrote(t *testing.T) {
	assert.Equal(t, "a single side of s or other.", Reword("exactly one of s or other."))
	assert.Equal(t, "Goroutines contend", Reword("Two goroutines contend"))
}

// A word that merely contains a number word is a name, so the table's own
// matching leaves it alone.
func TestTheTableLeavesAWordContainingANumberWordAlone(t *testing.T) {
	assert.Equal(t, "The oneShot flag and someone else", Reword("The oneShot flag and someone else"))
}

// A dot against the word after it opens a name. Closing the space before it
// wrote "Tests for.github/scripts" across a repository of dotfile references.
func TestTheTableKeepsTheSpaceBeforeALeadingDot(t *testing.T) {
	for _, prose := range []string{
		"Tests for .github/scripts/register.sh, the step it runs",
		"The suite lives in .dats files",
		"Ignored by .gitignore already",
	} {
		assert.Equal(t, prose, Reword(prose))
	}
}

// The gap a deleted word leaves before closing punctuation still closes.
func TestTheTableClosesTheGapBeforeClosingPunctuation(t *testing.T) {
	assert.Equal(t, "It reserves a single slot.", Reword("It reserves one slot ."))
	assert.Equal(t, "It locks, then writes.", Reword("It locks , then writes ."))
}
>>>>>>> origin/master
