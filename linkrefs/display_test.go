package linkrefs

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeResolver answers without a checkout, so the suite never shells out to git
// and every case states the repository state it is written against.
type fakeResolver struct {
	repo     Repo
	found    bool
	base     string
	branches []string
	commits  []string
	// states is what each pull request is doing. An absent reference is a
	// lookup that could not answer, and must render no dot.
	states map[string]PullState
}

func (f fakeResolver) Repo() (Repo, bool)         { return f.repo, f.found }
func (f fakeResolver) DefaultBranch() string      { return f.base }
func (f fakeResolver) BranchExists(b string) bool { return slices.Contains(f.branches, b) }
func (f fakeResolver) CommitExists(s string) bool { return slices.Contains(f.commits, s) }

func (f fakeResolver) PullState(repo Repo, number string) PullState {
	return f.states[repo.Owner+"/"+repo.Name+"#"+number]
}

// withState is live() plus a pull request whose state is known.
func withState(key string, s PullState) fakeResolver {
	res := live()
	res.states = map[string]PullState{key: s}
	return res
}

// live is a checkout of o/r whose master exists, with a pushed branch and a
// known commit.
func live() fakeResolver {
	return fakeResolver{
		repo:     Repo{Owner: "o", Name: "r"},
		found:    true,
		base:     "master",
		branches: []string{"claude/pushed"},
		commits:  []string{"6884dd2"},
	}
}

// bare is a directory that is not a checkout at all.
func bare() fakeResolver { return fakeResolver{} }

func rewrite(t *testing.T, text string, res Resolver) string {
	t.Helper()
	out, changed := RewriteDelta(text, false, res)
	if !changed {
		return text
	}
	return out
}

func TestAReferenceIsRenderedAsALink(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{
			"owner repo number needs no checkout",
			"wow-look-at-my/go-toolchain#376 landed.",
			"[wow-look-at-my/go-toolchain#376](https://github.com/wow-look-at-my/go-toolchain/issues/376) landed.",
		},
		{
			"a commit in this repository",
			"re-pushed as 6884dd2.",
			"re-pushed as [6884dd2](https://github.com/o/r/commit/6884dd2).",
		},
		{
			"a branch that is on the remote",
			"pushed claude/pushed to origin.",
			"pushed [claude/pushed](https://github.com/o/r/compare/master...claude/pushed?expand=1) to origin.",
		},
		{
			"a bare URL becomes clickable in place",
			"here: https://github.com/o/r/pull/30",
			"here: [https://github.com/o/r/pull/30](https://github.com/o/r/pull/30)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, rewrite(t, tc.text, live()))
		})
	}
}

// The rule this serves bans a dead link outright: a link is a demand on the
// reader's attention, paid before they know whether it was worth paying.
func TestAReferenceWithNoPageIsLeftAlone(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"a branch that was never pushed", "working on claude/never-pushed now."},
		{"a commit this repository does not have", "master is at e3665a4689bb now."},
		{"a branch with no repository to resolve against", "pushed claude/pushed to origin."},
	}
	res := []Resolver{live(), live(), bare()}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed := RewriteDelta(tc.text, false, res[i])
			assert.False(t, changed, "expected no rewrite, got %q", out)
		})
	}
}

// Each live state renders its dot beside a reference that is still linked, so
// the reader can tell what the page is doing without opening it.
func TestEachLiveStateRendersItsDot(t *testing.T) {
	cases := []struct {
		name  string
		state PullState
		want  string
	}{
		{"mergeable", StateMergeable, "🟢 [o/r#376](https://github.com/o/r/issues/376) is up."},
		{"checks running", StatePending, "🟡 [o/r#376](https://github.com/o/r/issues/376) is up."},
		{"checks failed", StateFailing, "🔴 [o/r#376](https://github.com/o/r/issues/376) is up."},
		{"conflicted", StateConflicted, "🟠 [o/r#376](https://github.com/o/r/issues/376) is up."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, rewrite(t, "o/r#376 is up.", withState("o/r#376", tc.state)))
		})
	}
}

// A merged pull request is the state where the link MOVES. The words go back to
// plain text and the purple dot carries the link. The open case beside it is the
// control that keeps them apart.
func TestMergedMovesTheLinkOntoTheDot(t *testing.T) {
	merged := rewrite(t, "o/r#376 landed.", withState("o/r#376", StateMerged))
	assert.Equal(t, "[🟣](https://github.com/o/r/issues/376) o/r#376 landed.", merged)

	open := rewrite(t, "o/r#376 landed.", withState("o/r#376", StateMergeable))
	assert.Equal(t, "🟢 [o/r#376](https://github.com/o/r/issues/376) landed.", open)

	assert.NotContains(t, merged, "[o/r#376](", "the words must not be a link when merged")
	assert.Contains(t, open, "[o/r#376](", "an open pull request keeps the reference linked")
}

// A pull request closed without merging is finished, so it renders the way a
// merged pull request does.
func TestAClosedPullRequestAlsoMovesTheLink(t *testing.T) {
	got := rewrite(t, "o/r#376 was dropped.", withState("o/r#376", StateClosed))
	assert.Equal(t, "[⚫](https://github.com/o/r/issues/376) o/r#376 was dropped.", got)
}

// A state that could not be determined renders NO dot. An unknown must never
// show as green.
func TestAnUnknownStateRendersNoDot(t *testing.T) {
	got := rewrite(t, "o/r#376 is up.", live())
	assert.Equal(t, "[o/r#376](https://github.com/o/r/issues/376) is up.", got)
	for _, d := range []string{"🟢", "🟡", "🔴", "🟠", "🟣", "⚫"} {
		assert.NotContains(t, got, d)
	}
}

// A bare pull request URL earns a dot too, and merged moves its link the same
// way. A trailing file path is ignored.
func TestABarePullRequestURLGetsItsDot(t *testing.T) {
	res := withState("o/r#376", StateFailing)
	assert.Equal(t,
		"here: 🔴 [https://github.com/o/r/pull/376](https://github.com/o/r/pull/376)",
		rewrite(t, "here: https://github.com/o/r/pull/376", res))

	merged := withState("o/r#376", StateMerged)
	assert.Equal(t,
		"diff: [🟣](https://github.com/o/r/pull/376/files) https://github.com/o/r/pull/376/files",
		rewrite(t, "diff: https://github.com/o/r/pull/376/files", merged))
}

// An answer about a pull request says nothing about any other.
func TestOnlyTheNamedReferenceGetsItsDot(t *testing.T) {
	res := live()
	res.states = map[string]PullState{"o/r#1": StateMerged, "o/r#2": StatePending}

	got := rewrite(t, "o/r#1 merged, o/r#2 is next.", res)
	assert.Equal(t,
		"[🟣](https://github.com/o/r/issues/1) o/r#1 merged, 🟡 [o/r#2](https://github.com/o/r/issues/2) is next.",
		got)
}

// A reference that is not a pull request is never asked about, and is linked the
// way it always was.
func TestANonPullRequestReferenceGetsNoDot(t *testing.T) {
	res := withState("o/r#376", StateMerged)
	for _, text := range []string{
		"re-pushed as 6884dd2.",
		"pushed claude/pushed to origin.",
		"tree: https://github.com/o/r/tree/claude/pushed",
	} {
		got := rewrite(t, text, res)
		assert.Contains(t, got, "](https://github.com/o/r/")
		assert.NotContains(t, got, "🟣", "for %q", text)
	}
}

// A backticked reference keeps its backticks inside the link text, and the dot
// sits outside the code span rather than inside it.
func TestABacktickedReferenceKeepsItsDotOutsideTheCodeSpan(t *testing.T) {
	got := rewrite(t, "see `o/r#376` for it.", withState("o/r#376", StateMergeable))
	assert.Equal(t, "see 🟢 [`o/r#376`](https://github.com/o/r/issues/376) for it.", got)
}

// An owner/repo#N slug carries its own repository, so it resolves even outside a
// checkout.
func TestASlugResolvesWithoutACheckout(t *testing.T) {
	got := rewrite(t, "see wow-look-at-my/dats#12 for the suite.", bare())
	assert.Equal(t, "see [wow-look-at-my/dats#12](https://github.com/wow-look-at-my/dats/issues/12) for the suite.", got)
}

// /issues/N and never /pull/N: GitHub redirects an issue number to its pull
// request, and /pull/N on a plain issue is an HTTP 404.
func TestNumbersUseTheSpellingThatWorksForBoth(t *testing.T) {
	got := rewrite(t, "o/r#42", live())
	assert.Contains(t, got, "/issues/42")
	assert.NotContains(t, got, "/pull/42")
}

// A bare #N is never linked. It cannot be checked before rendering, the
// repository it would resolve against is a guess, and it is the shape an
// ordinary numbered list uses.
func TestABareNumberIsNeverLinked(t *testing.T) {
	cases := []string{
		"PR #376 is green.",
		"- **#7** the sweep is still open",
		"#1 blocked, then #2 landed.",
	}
	for _, text := range cases {
		out, changed := RewriteDelta(text, false, live())
		assert.False(t, changed, "expected no rewrite of %q, got %q", text, out)
	}
}

func TestTextThatIsAlreadyLinkedIsNotRewrittenAgain(t *testing.T) {
	cases := []string{
		"[claude/pushed](https://github.com/o/r/compare/master...claude/pushed?expand=1) is up.",
		"[6884dd2](https://github.com/o/r/commit/6884dd2) fixes it.",
		"[o/r#376](https://github.com/o/r/pull/376) is merged.",
		"see <https://github.com/o/r/pull/1>",
	}
	for _, text := range cases {
		out, changed := RewriteDelta(text, false, live())
		assert.False(t, changed, "expected no rewrite of %q, got %q", text, out)
	}
}

// A URL contains a slug that the branch matcher also matches. Rewriting each
// would nest a link inside another link.
func TestAURLIsRewrittenOnceNotTwice(t *testing.T) {
	got := rewrite(t, "https://github.com/o/r/tree/claude/pushed", live())
	assert.Equal(t, 1, strings.Count(got, "]("), "expected exactly one link in %q", got)
}

func TestQuotedAndFencedLinesAreLeftAlone(t *testing.T) {
	text := "```\nPR #376 on claude/pushed\n```\n> quoted: PR #376\n    indented #376"
	out, changed := RewriteDelta(text, false, live())
	assert.False(t, changed, "expected no rewrite, got %q", out)
}

// A flush cannot see the ``` that opened earlier, so the state has to be
// carried in.
func TestFenceStateCarriesAcrossFlushes(t *testing.T) {
	out, changed := RewriteDelta("PR #376 inside the fence\n", true, live())
	assert.False(t, changed, "expected no rewrite inside a carried fence, got %q", out)

	assert.True(t, EndsInsideFence("intro\n```\ncode"))
	assert.False(t, EndsInsideFence("intro\n```\ncode\n```\nafter"))
}

// A reference wrapped in inline backticks must render with the backticks INSIDE
// the link text. Splicing over just the bare token leaves them straddling it,
// which markdown does not render as a link at all.
func TestABacktickWrappedReferenceKeepsTheBackticksInsideTheLink(t *testing.T) {
	res := fakeResolver{repo: Repo{Owner: "wow-look-at-my", Name: "slopfmt"}, found: true, commits: []string{"c4f997e"}}
	got := rewrite(t, "Resolved and pushed `c4f997e`.", res)
	assert.Equal(t, "Resolved and pushed [`c4f997e`](https://github.com/wow-look-at-my/slopfmt/commit/c4f997e).", got)
}

func TestEveryOccurrenceIsRewritten(t *testing.T) {
	got := rewrite(t, "o/r#1 blocked o/r#1 then o/r#2 landed.", live())
	assert.Equal(t, 3, strings.Count(got, "]("), "expected three links in %q", got)
}

// A delta with nothing to link reports no change, which leaves the reader
// looking at the original text.
func TestNothingToLinkReportsNoChange(t *testing.T) {
	for _, text := range []string{"", "the suite is green."} {
		_, changed := RewriteDelta(text, false, live())
		assert.False(t, changed, "for %q", text)
	}
}
