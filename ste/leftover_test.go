package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// Each sentence came out of this tree over the cap, with the repair asking for a rewrite by hand.
var leftovers = []string{
	"It goes only where an EARLIER entry in the same list already sent stdout to /dev/null, so the reversed spelling, which sends stderr to the terminal, is left alone.",
	"Asking the model to re-emit the message instead costs a round trip, and the message it writes to comply puts the decision back into prose while explaining itself.",
	"readText numbers the lines a Read call returns, the way the Read tool does: the line number, a tab, and the line with a trailing carriage return cut.",
	"Every entry is grounded in this org's own written convention, and the rest are direct synonyms of an entry already here, kept narrow rather than speculative.",
	"run runs git in dir with no user or system config, so a signing key or a hook on the machine cannot change what the test sees.",
	"fold repairs each run a block rule still reports on a line the fork wrote, where the fork's changes made the run longer than the base had it.",
	"A file the rule reports nothing in is written back byte for byte, so the cases above pass on a repair rather than on any edit.",
	"A prose finding carries the quantity so the repair can cut the cardinal off the front of it, which a list of phrases does not show.",
	"It names the repeated call, states the count, and gives the ways out, because a refusal that does not say what to do instead gets repeated with a different excuse.",
	"A card is right only where guessing is not: an action outside the branch, a destructive action, access this session lacks, or a fork the owner reserved.",
	"Package cardinal decides whether a number in a piece of text is a stated count: a number that is true today and wrong after the next commit.",
	"The same record written by an encoder that escapes HTML must read the same, or the guard is blind on half the transcripts it may be handed.",
	"A replacement that is absent, or that appears more than a single time, leaves the result unknown, and the caller then judges the fragment as it always did.",
	"The binary is a thin wrapper, so a hook, a CI job and an editor integration all get identical answers instead of separate implementations that drift.",
	"git reports the physical root, and on macOS a temp dir is reached through the /var symlink, so the cwd a session reports is spelled another way.",
	"A trailing run of turns that each made the exact same tool call, spaced close enough together that no real event or scheduled wakeup could plausibly explain the repeat.",
	"A comment that DOES document code is cut back to fit rather than deleted, so the case above is about the measure and not about comments.",
	"An entry may write more words than it matched, fewer, or none at all, so a rule can swap a word, delete it, or rewrite the phrase around it.",
	"Deciding whether a status read can still learn anything needs the RESULT text too, because that is where a merge or a green build is reported.",
	"The table is XML rather than Go, the same way autoallow carries its rules, so adding a rule is a single-line edit somebody can make without reading Go.",
	"A block that holds an earlier ending is repaired at that ending, which is the control that proves the cut is not the last line going.",
	"The closer is not prose: left in the text a rewrite wraps it into the middle of the comment, and dropped it leaves the block open and the file unparseable.",
}

func TestALeftoverSentenceDivides(t *testing.T) {
	for _, in := range leftovers {
		t.Run(in[:30], func(t *testing.T) {
			got := ste.Fix(in)
			for _, s := range ste.Sentences(got) {
				assert.LessOrEqual(t, ste.WordCount(s), ste.SentenceWordCap, "%q\n%s\n%s", got, syntax.Parse(in, nil).Tags(), syntax.Parse(in, nil).Outline())
				assert.True(t, ste.StandsAlone(s), "%q in %q\n%s\n%s", s, got, syntax.Parse(s, nil).Tags(), syntax.Parse(s, nil).Outline())
			}
		})
	}
}
