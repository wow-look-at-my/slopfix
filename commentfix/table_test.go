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

// What the table writes for a whole line lives in the rules folder, driven by
// the cases test beside this file.
