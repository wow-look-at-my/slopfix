package ste

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// Each sentence holds a clause boundary that ends a whole sentence on each side.
var clauseBoundaries = []string{
	"git reports the physical root, and on macOS a temp dir is reached through the /var symlink, so the cwd a session reports is spelled another way.",
	"An entry may write more words than it matched, fewer, or none at all, so a rule can swap a word, delete it, or rewrite the phrase around it.",
	"run runs git in dir with no user or system config, so a signing key or a hook on the machine cannot change what the test sees.",
	"The same record written by an encoder that escapes HTML must read the same, or the guard is blind on half the transcripts it may be handed.",
	"A replacement that is absent, or that appears more than a single time, leaves the result unknown, and the caller then judges the fragment as it always did.",
}

// TestAClauseBoundaryDivides names the gate that refuses each candidate, so a
// failure says where the division dies.
func TestAClauseBoundaryDivides(t *testing.T) {
	for _, in := range clauseBoundaries {
		t.Run(in[:30], func(t *testing.T) {
			masked := mask(in)
			s := syntax.Parse(masked, opaque(in, masked))
			_, ok := bestDivision(s, in)
			assert.True(t, ok, "%s\n%s\n%s", s.Tags(), s.Outline(), traceDivisions(s, in))
		})
	}
}

// traceDivisions writes each candidate division and the gate that refuses it.
func traceDivisions(s *syntax.Sentence, source string) string {
	var b strings.Builder
	for k := 1; k < len(s.Clauses); k++ {
		c := s.Clauses[k]
		main, ok := mainBefore(s, k)
		fmt.Fprintf(&b, "clause %d %q link=%d verb=%v main=%v", k, s.Span(c.First, c.Last), c.Link, c.Verb != nil, ok)
		if ok && c.Link >= 0 {
			opener, fits := openerFor(s, c, main, source)
			fmt.Fprintf(&b, " opener=%q fits=%v", opener, fits)
			if c.Kind == syntax.Coordinate && c.Subject != nil && c.Verb != nil {
				fmt.Fprintf(&b, " subjectFollows=%v agrees=%v capital=%v", subjectFollows(s, c), agrees(s, *c.Subject, *c.Verb), opensWithCapital(s, c.Link+1, source))
			}
		}
		b.WriteString("\n")
	}
	for _, d := range divisions(s, source) {
		right := joinOpener(d.opener, source[d.rightStart:])
		fmt.Fprintf(&b, "division at %d: admissible=%v stands=%v right=%q\n", d.leftEnd, admissible(s, source, d), standsAsSentence(right), right)
	}
	return b.String()
}
