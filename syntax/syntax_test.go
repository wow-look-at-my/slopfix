package syntax_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/syntax"
)

func subject(s *syntax.Sentence, c syntax.Clause) string {
	if c.Subject == nil {
		return ""
	}
	return s.Span(c.Subject.First, c.Subject.Last)
}

func verb(s *syntax.Sentence, c syntax.Clause) string {
	if c.Verb == nil {
		return ""
	}
	return s.Span(c.Verb.First, c.Verb.Last)
}

func parse(t *testing.T, text string) *syntax.Sentence {
	t.Helper()
	s := syntax.Parse(text, nil)
	t.Log(s.Tags())
	t.Log(s.Brackets())
	t.Log("\n" + s.Outline())
	return s
}

func TestAMainClauseFindsItsSubjectAndVerb(t *testing.T) {
	s := parse(t, "The gate reads every file in the session.")
	require.Len(t, s.Clauses, 1)
	assert.Equal(t, "The gate", subject(s, s.Clauses[0]))
	assert.Equal(t, "reads", verb(s, s.Clauses[0]))
}

func TestAVerbGroupAfterAndSharesTheSubject(t *testing.T) {
	s := parse(t, "The gate reads every file and refuses the write.")
	require.Len(t, s.Clauses, 2)
	c := s.Clauses[1]
	assert.Equal(t, syntax.Coordinate, c.Kind)
	assert.Nil(t, c.Subject)
	assert.Equal(t, "refuses", verb(s, c))
	assert.Zero(t, c.Depth)
}

func TestAClauseAfterAndCarriesItsOwnSubject(t *testing.T) {
	s := parse(t, "The gate reads every file and the write fails.")
	require.Len(t, s.Clauses, 2)
	assert.Equal(t, syntax.Coordinate, s.Clauses[1].Kind)
	assert.Equal(t, "the write", subject(s, s.Clauses[1]))
	assert.Equal(t, "fails", verb(s, s.Clauses[1]))
}

func TestNounsJoinedByAndAreOneClause(t *testing.T) {
	s := parse(t, "The gate reads files and directories.")
	assert.Len(t, s.Clauses, 1)
}

func TestARelativeClauseAfterACommaHangsBelowTheMainClause(t *testing.T) {
	s := parse(t, "The loader reads every row into memory, which is slow.")
	require.Len(t, s.Clauses, 2)
	c := s.Clauses[1]
	assert.Equal(t, syntax.Relative, c.Kind)
	assert.True(t, c.Comma)
	assert.Equal(t, 1, c.Depth)
	assert.Nil(t, c.Subject, "the relative word is the subject")
	assert.Equal(t, "is", verb(s, c))
}

func TestASubordinateClauseOpeningTheSentenceReturnsToTheMainClause(t *testing.T) {
	s := parse(t, "When the cache is cold, the build fails.")
	require.Len(t, s.Clauses, 2)
	assert.Equal(t, syntax.Subordinate, s.Clauses[0].Kind)
	assert.Equal(t, 1, s.Clauses[0].Depth)
	assert.Equal(t, "the cache", subject(s, s.Clauses[0]))
	assert.Equal(t, syntax.Opens, s.Clauses[1].Kind)
	assert.Zero(t, s.Clauses[1].Depth)
	assert.Equal(t, "the build", subject(s, s.Clauses[1]))
}

func TestBecauseOpensASubordinateClause(t *testing.T) {
	s := parse(t, "The build fails because the cache is cold.")
	require.Len(t, s.Clauses, 2)
	assert.Equal(t, syntax.Subordinate, s.Clauses[1].Kind)
	assert.Equal(t, "the cache", subject(s, s.Clauses[1]))
}

func TestAPrepositionIsNotASubordinator(t *testing.T) {
	s := parse(t, "The build fails after the merge.")
	assert.Len(t, s.Clauses, 1)
}

func TestAnImperativeHasNoSubject(t *testing.T) {
	s := parse(t, "Write a period and start a new sentence.")
	require.Len(t, s.Clauses, 2)
	require.NotNil(t, s.Clauses[0].Verb)
	assert.True(t, s.Clauses[0].Verb.Imperative)
	assert.Equal(t, syntax.Coordinate, s.Clauses[1].Kind)
}

func TestAPrepositionalPhraseStaysOnItsSubject(t *testing.T) {
	s := parse(t, "The file in the cache holds the tree.")
	require.Len(t, s.Clauses, 1)
	assert.Equal(t, "The file", subject(s, s.Clauses[0]))
}

func TestANumeralSitsBetweenTheDeterminerAndTheNoun(t *testing.T) {
	s := parse(t, "The three rules run on every file.")
	nps := s.NounPhrases()
	require.NotEmpty(t, nps)
	np := nps[0]
	assert.Equal(t, "The", s.Words[np.Det].Text)
	require.Len(t, np.Numerals, 1)
	assert.Equal(t, "three", s.Words[np.Numerals[0]].Text)
	assert.Equal(t, "rules", s.Words[np.Head].Text)
	assert.True(t, s.Plural(np))
}

func TestAPossessorIsTheDeterminerOfTheLongerPhrase(t *testing.T) {
	s := parse(t, "The repository's four branches build.")
	np := s.NounPhrases()[0]
	assert.Equal(t, "'s", s.Words[np.Det].Text)
	require.Len(t, np.Numerals, 1)
	assert.Equal(t, "branches", s.Words[np.Head].Text)
}

func TestANumeralWithNoNounIsTheHead(t *testing.T) {
	s := parse(t, "The two are equal.")
	np := s.NounPhrases()[0]
	assert.Empty(t, np.Numerals)
	assert.Equal(t, "two", s.Words[np.Head].Text)
}

func TestAnOpaqueSpanReadsAsAName(t *testing.T) {
	text := "The xxxxxxx flag stops the run."
	s := syntax.Parse(text, [][]int{{4, 11}})
	assert.Equal(t, "NNP", s.Words[1].Tag)
	require.Len(t, s.Clauses, 1)
	assert.Equal(t, "stops", verb(s, s.Clauses[0]))
}

func TestPersonNounsTakeThey(t *testing.T) {
	s := parse(t, "The caller waits.")
	np := s.NounPhrases()[0]
	assert.True(t, s.Person(np))
	assert.False(t, s.Plural(np))
}
