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
		"a so that states a purpose stays",
		"Your task is to produce a faithful, concise summary of the conversation so far so that a successor assistant can continue the work seamlessly after the earlier turns are discarded.",
		"",
	},
	{
		"a dash aside keeps its list whole",
		"Capture what is needed to continue — the user's explicit requests, your most recent actions, key technical details, file paths, commands, configuration, and architectural decisions — but be economical: prefer tight prose and short references over long verbatim dumps, and do not pad.",
		"",
	},
	{
		"a so before an imperative stays",
		"A focused summary that fits is far more useful than an exhaustive one that gets cut off, so aim for at most a few thousand words.",
		"",
	},
	{
		"an If clause keeps its main clause",
		"CRITICAL: If earlier turns include a prior compaction summary (marked with <conversation_summary> tags or a \"This session is being continued\" preamble), treat it as authoritative for the early history and carry its still-relevant information forward into your new summary so nothing important is lost across successive compactions.",
		"",
	},
	{
		"a parenthesis is no sentence",
		"For each, give the full path, why it matters, and the relevant code — include full snippets of any code you wrote or changed (with the most recent edits in full), not just descriptions.",
		"",
	},
	{
		"a so inside an instruction states its purpose",
		"When a next step exists, include a direct verbatim quote from the most recent messages showing exactly what you were doing and where you left off, so the task is interpreted without drift.",
		"",
	},
	{
		"an If clause before a comma is no sentence",
		"If the prior conversation contains a note about files at /tmp/compaction/segment_*.md or /tmp/compaction/INDEX.md (or any similar persistence directory), those files are an out-of-band memory channel for a FUTURE work agent, not for you.",
		"",
	},
	{
		"a so of result after a statement drops",
		"The file watcher reindexes the change on the next memory search, so the new entry is searchable within the current session.",
		"The file watcher reindexes the change on the next memory search. The new entry is searchable within the current session.",
	},
	{
		"then and a verb is no sentence",
		"When using `auth_provider_command`, you do not need to run `grok login` before starting — on first launch Grok runs your binary on the real terminal (URL and progress on stderr), then opens the UI already signed in.",
		"",
	},
	{
		"a prepositional phrase is no sentence",
		"To distribute MCP servers to a team, or to restrict which servers users can run (`allowedMcpServers` / `deniedMcpServers` in `requirements.toml` / `managed_config.toml`, with Claude `managed-settings.json` advisory for foreign-defined servers), see [Distribute across an organization](09-plugins.md#distribute-across-an-organization) in the Plugins guide.",
		"",
	},
	{
		"an adverb phrase is no sentence",
		"`[model_providers.ollama]` and `[model_providers.lmstudio]` with nothing in them are complete configurations: `apply_builtin_preset` fills the base URL, the dialect and the pricing switch from the id, and only where the user left them unset.",
		"`[model_providers.ollama]` and `[model_providers.lmstudio]` with nothing in them are complete configurations. `apply_builtin_preset` fills the base URL, the dialect and the pricing switch from the id, and only where the user left them unset.",
	},
	{
		"once more is no sentence",
		"Grok runs the command before a chat turn when the token is missing or within about a minute of expiring, and once more after the server rejects a token.",
		"",
	},
	{
		"a colon before a relative clause on the subject",
		"Exiting promptly on `GROK_AUTH_EXPIRED=1` is what makes the handover to the sign-in screen fast: a binary that blocks instead pays the whole refresh timeout on every start with an expired token.",
		"",
	},
	{
		"a pronoun between a noun and a verb opens another clause",
		"With no trip count given, each loop is modeled as one iteration **and the estimate is flagged** with a section in the report and a note in the output so it is never read as the exact cost.",
		"",
	},
	{
		"a participle before a noun is no verb",
		"[Always-approve](#always-approve) short-circuits this pipeline after step 2: `deny` rules, hooks, and `ask` rules that match a shell command's segments still apply, but remembered grants (including remembered \"never allow\" entries) are not consulted, and `ask` rules on non-shell tools do not prompt.",
		"",
	},
	{
		"a list of verbs keeps its items",
		"The agent dashboard now shows each agent's model and mode in the peek panel, lets you cycle modes with Shift+Tab, collapses the Inactive section by default, and hides older idle agents behind a \"N more\" row.",
		"",
	},
	{
		"a compound subject stays whole",
		"Never ask it to show, extract, quote or summarize a run or its transcript: RUN_LOG and your own investigation are the evidence, and gathering it is your job alone.",
		"",
	},
	{
		"a verb after a closing dash returns past the aside",
		"It is deliberately scoped to `go: ` lines rather than \"any duration under 1s anywhere in the log\": go-toolchain's own named step/test timers (e.g. `vet: gofmt 0.17s`) are intentionally unconditional — a named operation's own time is always worth reporting — and must not be flagged.",
		"It is deliberately scoped to `go: ` lines rather than \"any duration under 1s anywhere in the log\". Go-toolchain's own named step/test timers (e.g. `vet: gofmt 0.17s`) are intentionally unconditional — a named operation's own time is always worth reporting — and must not be flagged.",
	},
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
