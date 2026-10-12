package cmd

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
)

// Subcommands are shared, and driving them writes their streams and flags.
var cmdMu sync.Mutex

// find returns the registered subcommand of that name.
func find(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, c := range rootCmd.Commands() {
		// An alias is a name a caller may still exec, so it answers here too.
		if c.Name() == name || slices.Contains(c.Aliases, name) {
			return c
		}
	}
	t.Fatalf("no subcommand named %q is registered", name)
	return nil
}

// The binary carries commands. Cobra adds completion and help itself when it
// executes, so those below are every command this package registers.
func TestTheOnlyCommandsAreCheckHookLspAndRatchet(t *testing.T) {
	var names []string
	for _, c := range rootCmd.Commands() {
		if c.Name() != "completion" && c.Name() != "help" {
			names = append(names, c.Name())
		}
	}
	slices.Sort(names)
	assert.Equal(t, []string{"check", "hook", "lsp", "ratchet"}, names)
}

// guarded runs hook with only the named guards, the way --only selects them.
func guarded(t *testing.T, payload string, only ...string) hookResult {
	t.Helper()
	running, write, err := hookSelection(only)
	require.NoError(t, err)
	write.forks = noForks
	return dispatch([]byte(payload), running, write)
}

// writeEnvelope builds a PreToolUse payload for a Write of a file.
func writeEnvelope(path, content string) string {
	return envelope(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input":      map[string]any{"file_path": path, "content": content},
	})
}

// bash builds a PreToolUse payload for a shell command.
func bash(command string) string {
	return envelope(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": command},
	})
}

// display builds a MessageDisplay payload for a finished message. A guard keys
// its per-message state by the id, so each case brings its own.
func display(id, delta string) string {
	return envelope(map[string]any{
		"hook_event_name": "MessageDisplay",
		"message_id":      id,
		"final":           true,
		"delta":           delta,
	})
}

func envelope(payload map[string]any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// Each guard answers the input it exists to catch. A guard that exits clean in
// silence installs, runs and enforces nothing, while every surface says it does.
func TestEveryGuardAnswersItsOwnInput(t *testing.T) {
	for _, c := range []struct {
		only    []string
		payload string
		want    string
	}{
		{only: []string{"counts"}, payload: writeEnvelope("docs/a.md", "It has three plugins.\n"), want: `"content":"It has plugins.\n"`},
		{only: []string{"tombstones"}, payload: writeEnvelope("x.go", "package p\n\n// This used to read the flag from the environment.\nfunc f() {}\n"), want: `"content":"package p\n\nfunc f() {}\n"`},
		{only: []string{"auto-allow"}, payload: bash("python3 -c 1"), want: `"permissionDecision":"deny"`},
		{only: []string{"clean-bash"}, payload: bash("docker compose restart web"), want: "up -d"},
		{only: []string{"no-work-loss"}, payload: bash("truncate -s 0 README.md"), want: `"permissionDecision":"deny"`},
		{only: []string{"link-refs"}, payload: display("guard-link-refs", "Fixed in PazerOP/foo#42."), want: "](https://github.com/PazerOP/foo/"},
		{only: []string{"ask-properly"}, payload: display("guard-ask-properly", "I fixed the parser. Your call whether to ship it."), want: "ask-properly"},
		{only: []string{"laziness"}, payload: envelope(map[string]any{"hook_event_name": "Stop", "last_assistant_message": "I found an off-by-one in the retry loop and left it alone."}), want: "continue"},
	} {
		t.Run(strings.Join(c.only, ","), func(t *testing.T) {
			res := guarded(t, c.payload, c.only...)
			answer := res.Stdout + res.Stderr
			require.NotEmpty(t, answer, "%v answered nothing", c.only)
			assert.Contains(t, answer, c.want)
		})
	}
}

// With no --only, every guard runs. The write guard still repairs the write.
func TestEveryGuardRunsByDefault(t *testing.T) {
	res := guarded(t, writeEnvelope("docs/a.md", "It has three plugins.\n"))
	assert.Contains(t, res.Stdout, `"content":"It has plugins.\n"`)

	res = guarded(t, bash("python3 -c 1"))
	assert.Contains(t, res.Stdout, `"permissionDecision":"deny"`)
}

// An event no guard serves leaves the call alone. Printing there is how a hook
// interferes with work it was never meant to judge.
func TestAnUnservedEventIsSilent(t *testing.T) {
	assert.Equal(t, hookResult{}, guarded(t, `{"hook_event_name":"SessionEnd","tool_name":"Bash","tool_input":{"command":"ls"}}`))
}

// A payload that does not parse leaves the call alone too. A guard that refuses
// what it could not read is worse than no guard.
func TestAnUnreadablePayloadIsSilent(t *testing.T) {
	assert.Equal(t, hookResult{}, guarded(t, "{ not json"))
}

// The refusal the Bash cleaner owes on a heredoc. The write tools are what the
// message has to name.
func TestCleanBashRefusesAHeredoc(t *testing.T) {
	res := guarded(t, bash("cat <<EOF\nhello\nEOF\n"), "clean-bash")
	var resp struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &resp))
	assert.Equal(t, "PreToolUse", resp.HookSpecificOutput.HookEventName)
	assert.Equal(t, "deny", resp.HookSpecificOutput.PermissionDecision)
	assert.NotEmpty(t, resp.HookSpecificOutput.PermissionDecisionReason)
}

// The control that proves the case above can fail.
func TestCleanBashLeavesAnOrdinaryCommandAlone(t *testing.T) {
	assert.NotContains(t, guarded(t, bash("git status"), "clean-bash").Stdout, `"permissionDecision":"deny"`)
}

// An unknown name is an error, because a guard that runs nothing reads as a
// clean call. The error names both kinds of entry.
func TestAnUnknownGuardIsAnError(t *testing.T) {
	_, _, err := hookSelection([]string{"nosuch"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clean-bash")
	assert.Contains(t, err.Error(), "tombstones")
}

// A rule name turns the write guard on, and no other guard.
func TestARuleNameRunsTheWriteGuardAlone(t *testing.T) {
	running, write, err := hookSelection([]string{"counts"})
	require.NoError(t, err)
	assert.True(t, running.Equal(set.Of(writeGuard)))
	assert.NotEmpty(t, write.rules)
}

// fake is a guard that answers what it is told, and records what it was given.
func fake(name string, answer hookResult, seen *[]string) guard {
	return guard{name: name, events: []string{eventPreToolUse, eventStop, eventMessageDisplay}, run: func(payload []byte, _ writeSelection) hookResult {
		*seen = append(*seen, string(payload))
		return answer
	}}
}

// A rewrite of the tool input reaches each guard after it, and the answer
// carries the last rewrite.
func TestARewriteReachesTheGuardsAfterIt(t *testing.T) {
	var seen []string
	rewrite := hookResult{Stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"command":"after"}}}`}
	note := hookResult{Stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"a note"}}`}
	res := preToolUse([]byte(bash("before")), []guard{fake("a", rewrite, &seen), fake("b", note, &seen)}, writeSelection{})

	require.Len(t, seen, 2)
	assert.Contains(t, seen[1], `"command":"after"`)
	out := decode(res.Stdout)
	spec, _ := out["hookSpecificOutput"].(map[string]any)
	assert.Equal(t, map[string]any{"command": "after"}, spec["updatedInput"])
	assert.Equal(t, "a note", spec["additionalContext"])
}

// The first refusal is the answer, and no guard after it runs.
func TestTheFirstRefusalIsTheAnswer(t *testing.T) {
	var seen []string
	refusal := hookResult{Stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"no"}}`}
	res := preToolUse([]byte(bash("ls")), []guard{fake("a", refusal, &seen), fake("b", hookResult{}, &seen)}, writeSelection{})
	assert.Equal(t, refusal, res)
	assert.Len(t, seen, 1)
}

// Refusals of a stop become one refusal that carries both reasons, in
// either of the shapes a Stop guard writes.
func TestStopRefusalsAreJoined(t *testing.T) {
	res := merge(eventStop, []hookResult{
		{Stderr: "continue", Code: 2},
		{},
		{Stdout: `{"decision":"block","reason":"CLAUDE.md is over budget"}`},
	})
	assert.Equal(t, 2, res.Code)
	assert.Contains(t, res.Stderr, "continue")
	assert.Contains(t, res.Stderr, "CLAUDE.md is over budget")
}

// The rewrite replaces the text, and each note follows it.
func TestTheReaderSeesTheRewriteAndEveryNote(t *testing.T) {
	var seen []string
	shown := func(text string) hookResult {
		return hookResult{Stdout: encode(map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": eventMessageDisplay, "displayContent": text}})}
	}
	res := messageDisplay([]byte(display("m", "See x.")), []guard{
		fake("rewrite", shown("See [x](u)."), &seen),
		fake("note", shown("See x.\n\n> one"), &seen),
		fake("quiet", hookResult{}, &seen),
	}, writeSelection{})
	spec, _ := decode(res.Stdout)["hookSpecificOutput"].(map[string]any)
	assert.Equal(t, "See [x](u).\n\n> one", spec["displayContent"])
}

// check --message judges a closing message with every message rule.
func TestCheckMessageRunsEveryMessageRule(t *testing.T) {
	for _, c := range []struct {
		only    []string
		message string
		want    string
	}{
		{only: []string{"blame"}, message: "The suite is red, but the failure is pre-existing.", want: "blame/deflection"},
		{only: []string{"laziness/punt"}, message: "I found an off-by-one in the retry loop and left it alone.", want: "laziness/punt"},
		{only: []string{"ask"}, message: "I fixed the parser. Your call whether to ship it.", want: "ask/prose-decision"},
		{message: "I found an off-by-one in the retry loop and left it alone.", want: "laziness/punt"},
	} {
		t.Run(strings.Join(c.only, ","), func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			cmd.SetIn(strings.NewReader(c.message))
			require.ErrorIs(t, checkMessageStdin(cmd, c.only, false, false), errFindings)
			assert.Contains(t, out.String(), c.want)
		})
	}
}

// The control: a clean message reports nothing and passes.
func TestACleanMessagePasses(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("I fixed the off-by-one in the retry loop and pushed it."))
	require.NoError(t, checkMessageStdin(cmd, nil, true, false))
	assert.Contains(t, out.String(), `"findings":[]`)
}

// fix --message cuts each reported sentence, writes the message, and passes.
func TestFixMessageCutsEachReportedSentence(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("I pushed the retry fix. Want me to fix the loader too?\n"))
	require.NoError(t, checkMessageStdin(cmd, nil, false, true))
	assert.Equal(t, "I pushed the retry fix.\n", out.String())
}

func TestAnUnknownMessageRuleIsAnError(t *testing.T) {
	_, err := messageSelection([]string{"ste"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "laziness/punt")
}
