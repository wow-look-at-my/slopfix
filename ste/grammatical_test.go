package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/ste"
)

// Sentences from a real repository on which the repair once wrote broken
// English: "As a result,", "This is" in front of a phrase, a fragment cut off
// a dash or an "If" clause. Each now comes out as grammatical sentences under
// the cap. An empty want is a sentence under the cap as written.
var realSentences = []struct {
	name, in, want string
}{
	{
		"a trailing adverbial after a so that clause goes behind a carrier",
		"Your task is to produce a faithful, concise summary of the conversation so far so that a successor assistant can continue the work seamlessly after the earlier turns are discarded.",
		"Your task is to produce a faithful, concise summary of the conversation so far so that a successor assistant can continue the work seamlessly. This happens after the earlier turns are discarded.",
	},
	{
		"a dash aside keeps its list whole",
		"Capture what is needed to continue — the user's explicit requests, your most recent actions, key technical details, file paths, commands, configuration, and architectural decisions — but be economical: prefer tight prose and short references over long verbatim dumps, and do not pad.",
		"Capture what is needed to continue — the user's explicit requests, your most recent actions, key technical details, file paths, commands, configuration, and architectural decisions. However, be economical: prefer tight prose and short references over long verbatim dumps, and do not pad.",
	},
	{
		"a so before an instruction drops",
		"A focused summary that fits is far more useful than an exhaustive one that gets cut off, so aim for at most a few thousand words.",
		"A focused summary that fits is far more useful than an exhaustive one that gets cut off. Aim for at most a few thousand words.",
	},
	{
		"an If clause keeps its main clause",
		"CRITICAL: If earlier turns include a prior compaction summary (marked with <conversation_summary> tags or a \"This session is being continued\" preamble), treat it as authoritative for the early history and carry its still-relevant information forward into your new summary so nothing important is lost across successive compactions.",
		"CRITICAL: If earlier turns include a prior compaction summary (marked with <conversation_summary> tags or a \"This session is being continued\" preamble), treat it as authoritative for the early history. Carry its still-relevant information forward into your new summary so nothing important is lost across successive compactions.",
	},
	{
		"an instruction after a dash opens a sentence",
		"For each, give the full path, why it matters, and the relevant code — include full snippets of any code you wrote or changed (with the most recent edits in full), not just descriptions.",
		"For each, give the full path, why it matters, and the relevant code. Include full snippets of any code you wrote or changed (with the most recent edits in full), not just descriptions.",
	},
	{
		"a so inside an instruction goes behind Do this",
		"When a next step exists, include a direct verbatim quote from the most recent messages showing exactly what you were doing and where you left off, so the task is interpreted without drift.",
		"Include a direct verbatim quote from the most recent messages showing exactly what you were doing and where you left off. Do this so the task is interpreted without drift. Do this when a next step exists.",
	},
	{
		"a main clause that points back keeps the If clause first",
		"If the prior conversation contains a note about files at /tmp/compaction/segment_*.md or /tmp/compaction/INDEX.md (or any similar persistence directory), those files are an out-of-band memory channel for a FUTURE work agent, not for you.",
		"Suppose the prior conversation contains a note about files at /tmp/compaction/segment_*.md or /tmp/compaction/INDEX.md (or any similar persistence directory). Then those files are an out-of-band memory channel for a FUTURE work agent, not for you.",
	},
	{
		"a so of result after a statement drops",
		"The file watcher reindexes the change on the next memory search, so the new entry is searchable within the current session.",
		"The file watcher reindexes the change on the next memory search. The new entry is searchable within the current session.",
	},
	{
		"a clause after a dash opens a sentence",
		"When using `auth_provider_command`, you do not need to run `grok login` before starting — on first launch Grok runs your binary on the real terminal (URL and progress on stderr), then opens the UI already signed in.",
		"When using `auth_provider_command`, you do not need to run `grok login` before starting. On first launch Grok runs your binary on the real terminal (URL and progress on stderr), then opens the UI already signed in.",
	},
	{
		"an infinitive of purpose goes behind Do this",
		"To distribute MCP servers to a team, or to restrict which servers users can run (`allowedMcpServers` / `deniedMcpServers` in `requirements.toml` / `managed_config.toml`, with Claude `managed-settings.json` advisory for foreign-defined servers), see [Distribute across an organization](09-plugins.md#distribute-across-an-organization) in the Plugins guide.",
		"See [Distribute across an organization](09-plugins.md#distribute-across-an-organization) in the Plugins guide. Do this to distribute MCP servers to a team, or to restrict which servers users can run (`allowedMcpServers` / `deniedMcpServers` in `requirements.toml` / `managed_config.toml`, with Claude `managed-settings.json` advisory for foreign-defined servers).",
	},
	{
		"an adverb phrase is no sentence",
		"`[model_providers.ollama]` and `[model_providers.lmstudio]` with nothing in them are complete configurations: `apply_builtin_preset` fills the base URL, the dialect and the pricing switch from the id, and only where the user left them unset.",
		"`[model_providers.ollama]` and `[model_providers.lmstudio]` with nothing in them are complete configurations. `apply_builtin_preset` fills the base URL, the dialect and the pricing switch from the id, and only where the user left them unset.",
	},
	{
		"once more after a when clause goes behind a carrier",
		"Grok runs the command before a chat turn when the token is missing or within about a minute of expiring, and once more after the server rejects a token.",
		"Grok runs the command before a chat turn. This happens when the token is missing or within about a minute of expiring, and once more after the server rejects a token.",
	},
	{
		"a colon before a relative clause on the subject",
		"Exiting promptly on `GROK_AUTH_EXPIRED=1` is what makes the handover to the sign-in screen fast: a binary that blocks instead pays the whole refresh timeout on every start with an expired token.",
		"Exiting promptly on `GROK_AUTH_EXPIRED=1` is what makes the handover to the sign-in screen fast. A binary that blocks instead pays the whole refresh timeout on every start with an expired token.",
	},
	{
		"a bold clause after and opens a sentence in bold",
		"With no trip count given, each loop is modeled as one iteration **and the estimate is flagged** with a section in the report and a note in the output so it is never read as the exact cost.",
		"With no trip count given, each loop is modeled as one iteration. **The estimate is flagged** with a section in the report and a note in the output so it is never read as the exact cost.",
	},
	{
		"a participle before a noun is no verb",
		"[Always-approve](#always-approve) short-circuits this pipeline after step 2: `deny` rules, hooks, and `ask` rules that match a shell command's segments still apply, but remembered grants (including remembered \"never allow\" entries) are not consulted, and `ask` rules on non-shell tools do not prompt.",
		"[Always-approve](#always-approve) short-circuits this pipeline after step 2. `deny` rules, hooks, and `ask` rules that match a shell command's segments still apply, but remembered grants (including remembered \"never allow\" entries) are not consulted. `ask` rules on non-shell tools do not prompt.",
	},
	{
		"a list of verbs keeps its verbs behind the subject",
		"The agent dashboard now shows each agent's model and mode in the peek panel, lets you cycle modes with Shift+Tab, collapses the Inactive section by default, and hides older idle agents behind a \"N more\" row.",
		"The agent dashboard now shows each agent's model and mode in the peek panel and lets you cycle modes with Shift+Tab. The agent dashboard also collapses the Inactive section by default, and hides older idle agents behind a \"N more\" row.",
	},
	{
		"a compound subject stays whole",
		"Never ask it to show, extract, quote or summarize a run or its transcript: RUN_LOG and your own investigation are the evidence, and gathering it is your job alone.",
		"Never ask it to show, extract, quote or summarize a run or its transcript. RUN_LOG and your own investigation are the evidence, and gathering it is your job alone.",
	},
	{
		"a verb after a closing dash returns past the aside",
		"It is deliberately scoped to `go: ` lines rather than \"any duration under 1s anywhere in the log\": go-toolchain's own named step/test timers (e.g. `vet: gofmt 0.17s`) are intentionally unconditional — a named operation's own time is always worth reporting — and must not be flagged.",
		"It is deliberately scoped to `go: ` lines rather than \"any duration under 1s anywhere in the log\". Go-toolchain's own named step/test timers (e.g. `vet: gofmt 0.17s`) are intentionally unconditional — a named operation's own time is always worth reporting — and must not be flagged.",
	},
	{
		"a phrase of place after a noun in a sentence with no verb restates the noun",
		"The quick brown fox with the long red tail and the tiny black paws near the wooden barn behind the tall green hills of the northern valley beside the cold river under the grey winter sky.",
		"The quick brown fox with the long red tail and the tiny black paws near the wooden barn. That barn is behind the tall green hills of the northern valley beside the cold river under the grey winter sky.",
	},
}

// Sentences from this repository's own comments on which the repair wrote a
// fragment. Each pair names words the output must not hold.
var brokenShapes = []struct {
	name, in, never string
}{
	{
		"a preposition the verb ends on stays with it",
		"Nothing is refused and nothing is sent back to the model, because the reader is the person the question was aimed at: a prose question the reader can see marked as an offloaded decision has already cost the writer what it was meant to cost.",
		"aimed. This",
	},
	{
		"a preposition with no object stays with its verb",
		"questions.go finds the shapes a closing message must not end on: a question put to the user in prose, and a deferral that hands the user a decision without asking it through AskUserQuestion.",
		"end. This",
	},
	{
		"a verb keeps its object",
		"A session that asks the same question several ways -- `gh wait-ci`, then `gh wait-ci checks`, then `pull_request_read` -- produces differing signatures and no streak.",
		"asks. The",
	},
	{
		"a verb keeps its complement",
		"Package cardinal decides whether a number in a piece of text is a stated count: a number that is true today and wrong after the next commit.",
		"is. A",
	},
	{
		"a verb keeps its object phrase",
		"The tree names the lines to weigh, so a line of code that merely opens with a marker's characters is never mistaken for an empty comment.",
		"mistaken. This",
	},
	{
		"a long subject keeps its relative clause",
		"The negative control. A file the rule reports nothing in is written back byte for byte, so the cases above pass on a repair rather than on any edit.",
		"That file nothing",
	},
	{
		"a finite verb keeps the object right after it",
		"The first word of a sentence names an item only in lower case or before a finite verb, because a capital there opens an instruction: \"Run 3 tests\".",
		"names. An",
	},
	{
		"a negation never ends a phrase",
		"A block that holds an earlier ending is repaired at that ending, which is the control that proves the cut is not the last line going.",
		"not. The",
	},
	{
		"a copula keeps its complement",
		"A block that holds an earlier ending is repaired at that ending, which is the control that proves the cut is not simply the last line going.",
		"which is. The",
	},
	{
		"a main verb keeps a noun phrase with a relative clause",
		"semicolonDivides reports whether the semicolon at word i of s ends a sentence that opens at word from, and the words after it open a sentence of their own.",
		"ends. A",
	},
	{
		"a head never ends inside a whether clause",
		"opensSentenceAt reports whether the words after the mark at word i open with their own subject and a verb that agrees with it, with an imperative, or with a subordinate clause and then its main clause.",
		"words. This",
	},
	{
		"a phrase a later verb follows stays with its subject",
		"The edit tools are the sanctioned route and are left alone, except where their target is the live settings, which is how a session would re-grant what every rule above denies.",
		"That rule is",
	},
	{
		"an adverbial inside a relative clause stays there",
		"sed and awk are the filters that can write a file without being told to on the argv: sed's `w` command and awk's `print >` both name their target inside the program text.",
		"This holds without",
	},
	{
		"a reason keeps the main clause's coordinated verb",
		"preserved asserts a command was allowed because its at-risk content was committed into a preservation ref rather than lost, and returns the notice text for the caller to inspect further.",
		"This holds for",
	},
	{
		"a compound subject after so is no list",
		"The binary is a thin wrapper, so a hook, a CI job and an editor integration all get identical answers instead of separate implementations that drift.",
		"also is",
	},
	{
		"a clause after a colon is no list",
		"A word longer than the width goes on its own line rather than being broken: a URL or an identifier split across lines stops being either.",
		"This covers",
	},
	{
		"an -ing noun stays in its compound noun",
		"The fix pass repaired a comment's length in the source file and left the ste finding beside it for a reviewer to hit on the next pass through the tree.",
		"That ste is",
	},
}

func TestABrokenShapeComesOutWhole(t *testing.T) {
	for _, c := range brokenShapes {
		t.Run(c.name, func(t *testing.T) {
			got := ste.Fix(c.in)
			assert.NotContains(t, got, c.never, "%q", got)
		})
	}
}

func TestARealSentenceComesOutGrammaticalOrAsWritten(t *testing.T) {
	only := func(id string) bool { return id == ste.IDSentenceCap || id == ste.IDCommaSplice }
	for _, c := range realSentences {
		t.Run(c.name, func(t *testing.T) {
			want := c.want
			if want == "" {
				want = c.in
			}
			got := ste.FixSelected(c.in, only)
			assert.Equal(t, want, got)
			assert.NotContains(t, got, "As a result")
			assert.Equal(t, strings.Count(c.in, "This is"), strings.Count(got, "This is"), "the repair writes no filler")
			for _, f := range ste.Check(got, 1) {
				assert.NotEqual(t, ste.IDSentenceCap, f.ID, "every sentence comes out under the cap: %s", f.Detail)
			}
		})
	}
}
