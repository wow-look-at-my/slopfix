package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// These are the cases the ste-lint action asserted before it became a wrapper
// around slopfix. Each names the count it pins, and slopfix must agree.
var steLintCases = []struct {
	name string
	text string
	want map[string]int
}{
	{"appositive-list", "The endpoint sends malformed SSE, a 200 that is not an event stream, usage that\nnever arrives, or JSON with a truncated tail.\n\nA reader cannot tell whether the second item renames the first or stands beside\nit. Rule 5.3 wants one instruction per sentence. That ambiguity is what the\nrule exists to remove.\n",
		map[string]int{ste.IDCommaSplice: 1}},
	{"banned-modal", "If they diverge, the model gets a world that contradicts itself. `read_file`\nsucceeds on a path that `cat` reports as nonexistent, or `write_file` refuses a\npath the sandbox would happily accept.\n",
		map[string]int{ste.IDModal: 1}},
	{"code-span-opens-a-sentence", "Its order is deny, hooks, allow, mode, then prompt.\n`02-contracts.md` fixes it there, so those later milestones have a socket to\nplug into rather than a negotiation to have.\n",
		map[string]int{ste.IDSentenceCap: 0}},
	{"introductory-phrase", "The design rendered inline into scrollback, and that mattered then. Under the\nalt screen, there is no scrollback to select from. In that case, the answer is\nno. On a resize, a reflow is queued.\n",
		map[string]int{ste.IDCommaSplice: 0}},
	{"manual-wrapping", "A paragraph on one line reports nothing, whatever its width.\n\nThis paragraph is wrapped by hand, so\nits second line reports a finding.\n\n- A list item wraps the same way,\n  and its continuation reports one too.\n\n| A table | is not prose |\n| --- | --- |\n| so a second row | reports nothing |\n",
		map[string]int{slopfix.IDHardWrap: 2, ste.IDSentenceCap: 0}},
	{"quoted-owner-line", "The owner ruled on reloading a file the harness itself wrote:\n\n> *\"change didn't happen due to ourselves using write/edit, when we reload it\n> have some kind of marker; the model can't get to it.\"*\n\nA quotation is somebody else's words. The rule reads the document's own prose.\n",
		map[string]int{ste.IDContraction: 0, ste.IDSemicolon: 0}},
	{"quoted-span-then-semicolon", "Cancellation works because `--die-with-parent` \"kills (SIGKILL) all bwrap\nsandbox processes in sequence from parent to child … when bwrap or bwrap's parent dies\";\non macOS the process group is signalled directly.\n",
		map[string]int{ste.IDSemicolon: 1}},
	{"real-comma-splice", "The queue is a display, it never takes focus.\nA patch held in a vendor tree is not held, it is waiting.\n\nRule 5.3 counts the instructions, not the conjunctions, so the last line is a\nfinding as well.\n\nThe terminal is repainted from one goroutine, and it is never blocked by a worker.\n",
		map[string]int{ste.IDCommaSplice: 3}},
	{"comma-splice-after-a-parenthetical", "A downloaded analyzer needs its engine beside it (buildhost project `vega-analyzer/spv2gcn-engine`), and a downloaded vkbench needs `vkb-engine` (buildhost project `vega-analyzer/vkb-engine`).\n",
		map[string]int{ste.IDCommaSplice: 1}},
	{"section-sign-opens-a-sentence", "The reply goes the long way round via `app.Answer`, and it arrives at the\ncomposer as an ordinary event rather than as a return value.\n§3a states that a prompt does not require a run.\n\nA section sign opens the second sentence. A checker that does not read it as a\nsentence start joins the two, and the joined pair is over the cap.\n",
		map[string]int{ste.IDSentenceCap: 0}},
	{"subordinate-clause", "If a rendering surface other than a terminal is ever wanted, it gets designed\nthen. When the run ends and nothing is queued, the hook is dispatched. Because\nthe value is unknown, it renders as a dash.\n",
		map[string]int{ste.IDCommaSplice: 0}},
	{"tense-across-a-wrap", "The value has\nbeen replaced by the loader.\n",
		map[string]int{ste.IDTense: 1}},
	{"wrapped-long-sentence", "The line count is what the checker used to measure, and prose here wraps near\nninety columns, so a sentence that runs well past the cap arrives as three\nshort fragments that each pass on their own.\n",
		map[string]int{ste.IDSentenceCap: 1}},
}

func TestSlopfixAgreesWithEverySteLintCase(t *testing.T) {
	for _, c := range steLintCases {
		t.Run(c.name, func(t *testing.T) {
			got := map[string]int{}
			var all []string
			for _, f := range slopfix.CheckContent("doc.md", c.text) {
				got[f.ID]++
				all = append(all, f.String())
			}
			for id, n := range c.want {
				assert.Equal(t, n, got[id], "%s: %v", id, all)
			}
		})
	}
}

// Each warning rule fires on prose that breaks it, warns rather than fails,
// and stays quiet on prose that keeps to it.
func TestEachWarningRuleFires(t *testing.T) {
	cases := map[string][2]string{
		ste.IDInstructionLength: {
			"Open the panel and remove the four screws that hold the cover before you disconnect the cable from the main board.",
			"Open the panel and remove the screws.",
		},
		ste.IDPassive:     {"The file is deleted by the loader.", "The loader deletes the file."},
		ste.IDNounCluster: {"The upload session finalize integrity check fails.", "The integrity check on the finalize fails."},
		ste.IDTense:       {"The loader is reading the file.", "The loader reads the file."},
		ste.IDDictionary:  {"The loader will accomplish the task.", "The loader will do the task."},
		ste.IDParagraphLength: {
			"It reads. It writes. It waits. It stops. It starts. It ends. It fails.",
			"It reads. It writes. It waits.",
		},
	}
	for id, texts := range cases {
		t.Run(id, func(t *testing.T) {
			var hits []ste.Finding
			for _, f := range slopfix.Warnings(texts[0] + "\n") {
				if f.ID == id {
					hits = append(hits, f)
				}
			}
			if assert.NotEmpty(t, hits, "%q", texts[0]) {
				assert.True(t, hits[0].Warning())
			}
			for _, f := range slopfix.Warnings(texts[1] + "\n") {
				assert.NotEqual(t, id, f.ID, "%q: %s", texts[1], f)
			}
		})
	}
}

// A list item never counts toward a paragraph's sentences.
func TestAListItemIsNotALongParagraph(t *testing.T) {
	for _, f := range slopfix.Warnings("- It reads. It writes. It waits. It stops. It starts. It ends. It fails.\n") {
		assert.NotEqual(t, ste.IDParagraphLength, f.ID)
	}
}
