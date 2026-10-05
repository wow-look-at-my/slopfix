package ste

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/syntax"
)

var probeSentences = []string{
	"The mock with id 1 completes after a short delay.",
	"It is slow. Then branch 3 sees the entry already cleared.",
	"id 1 completes after a short delay.",
	"With no trip count given, each loop is modeled as one iteration **and the estimate is flagged** with a section in the report and a note in the output so it is never read as the exact cost.",
	"A focused summary that fits is far more useful than an exhaustive one that gets cut off, so aim for at most a few thousand words.",
	"Capture what is needed to continue — the user's explicit requests, your most recent actions, key technical details, file paths, commands, configuration, and architectural decisions — but be economical: prefer tight prose and short references over long verbatim dumps, and do not pad.",
	"Exiting promptly on `GROK_AUTH_EXPIRED=1` is what makes the handover to the sign-in screen fast: a binary that blocks instead pays the whole refresh timeout on every start with an expired token.",
	"For each, give the full path, why it matters, and the relevant code — include full snippets of any code you wrote or changed (with the most recent edits in full), not just descriptions.",
	"When using `auth_provider_command`, you do not need to run `grok login` before starting — on first launch Grok runs your binary on the real terminal (URL and progress on stderr), then opens the UI already signed in.",
	"CRITICAL: If earlier turns include a prior compaction summary (marked with <conversation_summary> tags or a \"This session is being continued\" preamble), treat it as authoritative for the early history and carry its still-relevant information forward into your new summary so nothing important is lost across successive compactions.",
	"The agent dashboard now shows each agent's model and mode in the peek panel, lets you cycle modes with Shift+Tab, collapses the Inactive section by default, and hides older idle agents behind a \"N more\" row.",
	"To distribute MCP servers to a team, or to restrict which servers users can run (`allowedMcpServers` / `deniedMcpServers` in `requirements.toml` / `managed_config.toml`, with Claude `managed-settings.json` advisory for foreign-defined servers), see [Distribute across an organization](09-plugins.md#distribute-across-an-organization) in the Plugins guide.",
	"Never ask it to show, extract, quote or summarize a run or its transcript: RUN_LOG and your own investigation are the evidence, and gathering it is your job alone.",
	"[Always-approve](#always-approve) short-circuits this pipeline after step 2: `deny` rules, hooks, and `ask` rules that match a shell command's segments still apply, but remembered grants (including remembered \"never allow\" entries) are not consulted, and `ask` rules on non-shell tools do not prompt.",
	"A reader arriving at this paragraph without any conjunction anywhere inside its single enormous run-on clause still deserves a repair from the tool rather than a deletion.",
	"The quick brown fox with the long red tail and the tiny black paws near the wooden barn behind the tall green hills of the northern valley beside the cold river under the grey winter sky.",
}

func TestProbe(t *testing.T) {
	var b strings.Builder
	for _, in := range probeSentences {
		s := syntax.Parse(checkMask(in), nil)
		var tags []string
		for _, w := range s.Words {
			tags = append(tags, w.Text+"/"+w.Tag)
		}
		fmt.Fprintf(&b, "\nIN:   %s\nTAGS: %s\n", in, strings.Join(tags, " "))
		for _, c := range s.Clauses {
			subj, verb := "-", "-"
			if c.Subject != nil {
				subj = fmt.Sprint(c.Subject.First, "..", c.Subject.Last)
			}
			if c.Verb != nil {
				verb = fmt.Sprint(c.Verb.First, "..", c.Verb.Last, " imp=", c.Verb.Imperative)
			}
			fmt.Fprintf(&b, "  clause kind=%v depth=%d link=%d first=%d last=%d subj=%s verb=%s\n", c.Kind, c.Depth, c.Link, c.First, c.Last, subj, verb)
		}
		fmt.Fprintf(&b, "OUT:  %s\n", fixSentenceCap(in))
		if !strings.HasPrefix(in, "Capture") && !strings.HasPrefix(in, "A focused") && !strings.HasPrefix(in, "[Always") && !strings.HasPrefix(in, "The agent") {
			continue
		}
		masked := checkMask(in)
		whole := syntax.Parse(masked, nil)
		for _, strict := range []bool{true, false} {
			for _, c := range candidates(in, masked, strict) {
				right, opened := openRest(in, masked, whole, c)
				head := in[:c.left]
				if right == "" {
					head, right, opened = carrierDivision(in, whole, c)
				}
				if right == "" {
					continue
				}
				left := closeHead(head)
				fmt.Fprintf(&b, "  cut strict=%v score=%d opened=%d aside=%v divides=%v closes=%v phrase=%v | %s || %s\n", strict, c.score, opened,
					cutsAside(masked, c.left, c.right), divides(left, right), closesWhole(in[:c.left], seamBefore(in, c.left), whole), closesPhrase(in[:c.left], whole), left, right)
			}
		}
	}
	t.Error(b.String())
}
