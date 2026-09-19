package table_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/table"
)

// compiled wires an entry the way rulegen does, with regexp standing in for the
// generated automaton. Under test is the driver: the scan, the boundary check
// and the group expansion.
func compiled(bare, to string, lead, tail bool) table.Pattern {
	has := regexp.MustCompile(bare)
	at := regexp.MustCompile(`^(?s)(` + bare + `)(?s:.*)$`)
	return table.Pattern{
		Match: bare, To: to,
		Has:      has.MatchString,
		At:       at.MatchString,
		Find:     at.FindStringSubmatchIndex,
		LeadWord: lead, TailWord: tail,
	}
}

func TestATextWithNoMatchIsReturnedAsItIs(t *testing.T) {
	p := compiled(`one`, "a single", false, false)
	assert.Equal(t, "nothing here", p.Replace("nothing here"))
}

func TestEveryMatchIsRewritten(t *testing.T) {
	p := compiled(`one`, "a single", false, false)
	assert.Equal(t, "a single and a single", p.Replace("one and one"))
}

// The group expansion, in both spellings a table writes.
func TestAGroupIsWrittenIntoTheReplacement(t *testing.T) {
	assert.Equal(t, "a single slot", compiled(`one\s+([a-z])`, "a single ${1}", false, false).
		Replace("one slot"))
	assert.Equal(t, "slot", compiled(`two\s+([a-z])`, "$1", false, false).
		Replace("two slot"))
}

// A boundary the compiled matcher could not carry is the driver's to apply.
func TestALeadingBoundaryRejectsAMatchInsideAWord(t *testing.T) {
	p := compiled(`one`, "a single", true, false)
	assert.Equal(t, "someone", p.Replace("someone"))
	assert.Equal(t, "a single", p.Replace("one"))
}

func TestATrailingBoundaryRejectsAMatchInsideAWord(t *testing.T) {
	p := compiled(`one`, "a single", false, true)
	assert.Equal(t, "oneshot", p.Replace("oneshot"))
	assert.Equal(t, "a single call", p.Replace("one call"))
}

// The driver must answer as the regexp engine answers. The entries in rules/
// were written against that behaviour.
func TestTheDriverAgreesWithRegexp(t *testing.T) {
	for _, c := range []struct{ bare, to, in string }{
		{`one\s+([a-z])`, "a single ${1}", "It reserves one slot and one lock."},
		{`\s+([,.;:!?])`, "$1", "A clause , and a stop ."},
		{`  +`, " ", "Two  spaces  here"},
		{`([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#([0-9]+)`, "[$1#$2](x/$1/$2)", "a/b#14 and c/d#9"},
		{`(?i)the second`, "the next", "The second call, the second time"},
	} {
		want := regexp.MustCompile(c.bare).ReplaceAllString(c.in, c.to)
		assert.Equal(t, want, compiled(c.bare, c.to, false, false).Replace(c.in),
			"the driver and regexp disagree on %q", c.bare)
	}
}

// A replacement carrying no dollar is written as it stands.
func TestAPlainReplacementIsWrittenWhole(t *testing.T) {
	p := compiled(`first,\s+`, "", false, false)
	assert.Equal(t, "it locks", p.Replace("first, it locks"))
}

// An entry with no compiled matcher rewrites nothing rather than panicking. A
// table built by hand in a test is the case that reaches this.
func TestAnEntryWithNoMatcherIsInert(t *testing.T) {
	assert.Equal(t, "text", table.Pattern{To: "x"}.Replace("text"))
}

func TestAppliesToReadsTheSurface(t *testing.T) {
	assert.True(t, table.AppliesTo("", "comment"))
	assert.True(t, table.AppliesTo("both", "message"))
	assert.True(t, table.AppliesTo("message", "message"))
	assert.False(t, table.AppliesTo("message", "comment"))
}

// The prefilter is what keeps a run over ordinary prose free of allocation.
func TestAMissAllocatesNothing(t *testing.T) {
	p := compiled(`one\s+([a-z])`, "a single ${1}", false, false)
	line := strings.Repeat("the loader reads the file and waits for it. ", 8)
	assert.Equal(t, line, p.Replace(line))
}
