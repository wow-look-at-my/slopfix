package busypoll

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stageTranscript writes lines as a JSONL transcript and returns its path.
func stageTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	return path
}

// bashCall is an assistant record running a Bash command.
func bashCall(command string) string {
	input, _ := json.Marshal(map[string]string{"command": command})
	return assistantCall("Bash", string(input))
}

// assistantCall is an assistant record making a tool call.
func assistantCall(name, input string) string {
	return encodeLine(map[string]any{
		"type":      "assistant",
		"timestamp": "2026-09-05T01:00:00Z",
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "name": name, "input": json.RawMessage(input)},
		}},
	})
}

// encodeLine writes a fixture the way the harness writes it. Go's json.Marshal
// escapes `<` and JavaScript does not, so marshaling builds an unreal fixture.
func encodeLine(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimRight(b.String(), "\n")
}

// toolResult is the record a call's answer arrives in. It never starts a new
// turn, and it is where a verdict about a subject is reported.
func toolResult(text string) string {
	return encodeLine(map[string]any{
		"type":      "user",
		"timestamp": "2026-09-05T01:00:01Z",
		"message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "content": text},
		}},
	})
}

// userPrompt is a genuine new prompt, which re-opens every subject.
func userPrompt(text string) string {
	return encodeLine(map[string]any{
		"type":      "user",
		"timestamp": "2026-09-05T01:00:02Z",
		"message":   map[string]any{"role": "user", "content": text},
	})
}

// preToolPayload is the payload the harness sends before a call runs.
func preToolPayload(t *testing.T, transcript, tool, input string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"transcript_path": transcript,
		"tool_name":       tool,
		"tool_input":      json.RawMessage(input),
	})
	require.NoError(t, err)
	return string(b)
}

// bashInput is the tool_input a Bash call carries.
func bashInput(command string) string {
	b, _ := json.Marshal(map[string]string{"command": command})
	return string(b)
}

// denyReasonOf runs the hook and returns the refusal reason, or "" on allow.
func denyReasonOf(t *testing.T, payload string) string {
	t.Helper()
	res := Run(strings.NewReader(payload))
	require.Equal(t, 0, res.Code, "PreToolUse carries its verdict in stdout, never in an exit code")
	if res.Stdout == "" {
		return ""
	}
	var out preToolOutput
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out))
	require.Equal(t, "deny", out.HookSpecificOutput.PermissionDecision)
	require.Equal(t, "PreToolUse", out.HookSpecificOutput.HookEventName)
	return out.HookSpecificOutput.PermissionDecisionReason
}

func TestAMergedPullRequestIsNeverReadAgain(t *testing.T) {
	tr := stageTranscript(t,
		bashCall("gh wait-ci checks --repo wow-look-at-my/grok-build"),
		toolResult(`{"outcome":"merged","pr":"wow-look-at-my/grok-build#87"}`),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr view 87 --repo wow-look-at-my/grok-build")))

	require.NotEmpty(t, reason, "a merged pull request cannot answer differently")
	assert.Contains(t, reason, "wow-look-at-my/grok-build#87",
		"the refusal must name the subject it settled, not just say no")
	assert.Contains(t, reason, "is settled")
}

func TestADifferentPullRequestIsStillReadable(t *testing.T) {
	tr := stageTranscript(t,
		toolResult(`{"outcome":"merged","pr":"wow-look-at-my/grok-build#87"}`),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr view 88 --repo wow-look-at-my/grok-build")))

	assert.Empty(t, reason, "one pull request merging says nothing about another")
}

// callWithID is an assistant record whose tool_use carries an id, so a result
// can be tied back to it.
func callWithID(id, command string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant", "timestamp": "2026-09-05T01:00:00Z",
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": id, "name": "Bash",
				"input": map[string]string{"command": command}},
		}},
	})
	return string(b)
}

// describedCall is callWithID plus the `description` a real Bash call carries.
func describedCall(id, command, description string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant", "timestamp": "2026-09-05T01:00:00Z",
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": id, "name": "Bash",
				"input": map[string]string{"command": command, "description": description}},
		}},
	})
	return string(b)
}

// describedInput is the judged call's own input, description included.
func describedInput(command, description string) string {
	b, _ := json.Marshal(map[string]string{"command": command, "description": description})
	return string(b)
}

// resultFor is a call's answer. failed says whether it came back an error.
func resultFor(id, text string, failed bool) string {
	block := map[string]any{"type": "tool_result", "tool_use_id": id, "content": text}
	if failed {
		block["is_error"] = true
	}
	b, _ := json.Marshal(map[string]any{
		"type": "user", "timestamp": "2026-09-05T01:00:01Z",
		"message": map[string]any{"role": "user", "content": []any{block}},
	})
	return string(b)
}

// callIn and resultIn are stamped with the session that wrote the record, and
// with the sidechain flag a subagent's records carry.
func callIn(id, command, session string, sidechain bool) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant", "timestamp": "2026-09-05T01:00:00Z",
		"sessionId": session, "isSidechain": sidechain,
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": id, "name": "Bash",
				"input": map[string]string{"command": command}},
		}},
	})
	return string(b)
}

func resultIn(id, text, session string, sidechain bool) string {
	b, _ := json.Marshal(map[string]any{
		"type": "user", "timestamp": "2026-09-05T01:00:01Z",
		"sessionId": session, "isSidechain": sidechain,
		"message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": id, "content": text},
		}},
	})
	return string(b)
}

// preToolPayloadIn is preToolPayload carrying the session id the harness sends.
func preToolPayloadIn(t *testing.T, transcript, session, tool, input string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"transcript_path": transcript,
		"session_id":      session,
		"tool_name":       tool,
		"tool_input":      json.RawMessage(input),
	})
	require.NoError(t, err)
	return string(b)
}

func TestAnEarlierSessionsReadIsNotThisSessionsRead(t *testing.T) {
	// A resumed conversation writes into the same file, each record stamped.
	const read = "gh pr view 130 --repo wow-look-at-my/grok-build"
	tr := stageTranscript(t,
		callIn("t1", read, "session-before", false),
		resultIn("t1", `{"state":"open"}`, "session-before", false),
	)
	assert.Empty(t, denyReasonOf(t, preToolPayloadIn(t, tr, "session-now", "Bash", bashInput(read))),
		"another session's read is not an answer this session holds")

	same := stageTranscript(t,
		callIn("t1", read, "session-now", false),
		resultIn("t1", `{"state":"open"}`, "session-now", false),
	)
	assert.NotEmpty(t, denyReasonOf(t, preToolPayloadIn(t, same, "session-now", "Bash", bashInput(read))),
		"the control: this session's own answered read is still a repeat")
}

func TestASubagentsReadIsNotTheCallersRead(t *testing.T) {
	const read = "gh pr view 130 --repo wow-look-at-my/grok-build"
	tr := stageTranscript(t,
		callIn("t1", read, "session-now", true),
		resultIn("t1", `{"state":"open"}`, "session-now", true),
	)
	assert.Empty(t, denyReasonOf(t, preToolPayloadIn(t, tr, "session-now", "Bash", bashInput(read))),
		"a subagent's records share the transcript, and its answer is not the caller's")
}

func TestALocalGitCommandIsNeverAStatusRead(t *testing.T) {
	// Each reads a local object, reaches no network, and asks nothing's state.
	const sha = "4f7cea8b1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f60"
	tr := stageTranscript(t,
		callIn("t1", "gh wait-ci checks --sha "+sha, "s", false),
		resultIn("t1", "still running", "s", false),
	)
	for _, cmd := range []string{
		"git show " + sha + ":src/cmd/go.mod",
		"git ls-tree " + sha + " src/cmd/",
		"git log " + sha,
		"git ls-remote origin " + sha,
	} {
		assert.Empty(t, denyReasonOf(t, preToolPayloadIn(t, tr, "s", "Bash", bashInput(cmd))),
			"%s reads a local object and costs nothing", cmd)
	}

	assert.NotEmpty(t, denyReasonOf(t, preToolPayloadIn(t, tr, "s", "Bash",
		bashInput("gh wait-ci checks --sha "+sha))),
		"the control: asking GitHub the same question again is still a repeat")
}

func TestASHAInANeighbouringStatementIsNotTheSubject(t *testing.T) {
	// The earlier call asked GitHub, then read a local object further along.
	const sha = "4f7cea8b1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f60"
	tr := stageTranscript(t,
		callIn("t1", "gh pr view 130 && git show "+sha+":go.mod", "s", false),
		resultIn("t1", `{"state":"open"}`, "s", false),
	)
	assert.Empty(t, denyReasonOf(t, preToolPayloadIn(t, tr, "s", "Bash",
		bashInput("gh wait-ci checks --sha "+sha))),
		"the commit was never asked after, so this is its first read")

	assert.NotEmpty(t, denyReasonOf(t, preToolPayloadIn(t, tr, "s", "Bash",
		bashInput("gh pr checks 130"))),
		"the control: the pull request the gh statement did name is still settled")
}

func TestAReadThatErroredIsNotARead(t *testing.T) {
	const sha = "4f7cea8b1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f60"
	tr := stageTranscript(t,
		callWithID("t1", "gh wait-ci --sha "+sha+" --timeout 15m"),
		resultFor("t1", "ERROR: unknown flag: --timeout", true),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci --sha "+sha)))

	assert.Empty(t, reason, "a call that errored returned no state, so this is the first read")
}

// The result of a call is not on disk yet when the NEXT call's hook reads the
// transcript. Counting an unanswered read refused the retry of a command that
// had just died on an unknown flag.
func TestAReadWithNoResultYetIsNotARead(t *testing.T) {
	const sha = "4f7cea8b1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f60"
	tr := stageTranscript(t,
		callWithID("t1", "gh wait-ci --sha "+sha+" --timeout 15m"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci --sha "+sha)))

	assert.Empty(t, reason, "a call with no result carries no state, so this is the first read")
}

func TestAReadThatAnsweredIsARead(t *testing.T) {
	const sha = "4f7cea8b1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f60"
	tr := stageTranscript(t,
		callWithID("t1", "gh wait-ci --sha "+sha),
		resultFor("t1", "Progress: 3/8 still running", false),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci checks --sha "+sha)))

	require.NotEmpty(t, reason, "a read that answered is the repeat this guard exists for")
	assert.Contains(t, reason, "4f7cea8")
}

func TestAGreenCommitIsNeverReadAgain(t *testing.T) {
	tr := stageTranscript(t,
		bashCall("gh wait-ci --sha c274ad3c1a9c7bc156d706dc6062b2ab298417c0"),
		toolResult("Progress: 8/8\nCI PASSED\nCommit: c274ad3c1a9c7bc156d706dc6062b2ab298417c0"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci checks --sha c274ad3c1a9c7bc156d706dc6062b2ab298417c0")))

	require.NotEmpty(t, reason, "a commit that went green cannot go red")
	assert.Contains(t, reason, "c274ad3")
}

func TestAnotherCommitIsStillReadableAfterOneGoesGreen(t *testing.T) {
	tr := stageTranscript(t,
		toolResult("CI PASSED for c274ad3c1a9c7bc156d706dc6062b2ab298417c0"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci --sha d15b136aa1b2c3d4e5f60718293a4b5c6d7e8f90")))

	assert.Empty(t, reason, "a push makes a new commit, which is a new question")
}

// A record naming several commits says which of them went green no more than
// it says which did not. A green verdict sitting beside an unrelated sha used
// to settle that sha as well, so a commit still queued answered "settled".
func TestAGreenVerdictSettlesOnlyTheCommitItsRecordIsAbout(t *testing.T) {
	const green = "c274ad3c1a9c7bc156d706dc6062b2ab298417c0"
	const running = "d15b136aa1b2c3d4e5f60718293a4b5c6d7e8f90"
	tr := stageTranscript(t,
		bashCall("gh wait-ci runs --branch claude/work"),
		toolResult("CI PASSED "+green+"\nqueued "+running),
	)

	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci checks --sha "+running)))

	assert.Empty(t, reason, "the verdict was about the other commit in that listing")
}

func TestRereadingTheSameSubjectWithNothingInBetweenIsRefused(t *testing.T) {
	tr := stageTranscript(t,
		callWithID("t1", "gh pr view 87 --repo wow-look-at-my/grok-build"),
		resultFor("t1", `{"pr":"wow-look-at-my/grok-build#87","state":"open"}`, false),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr checks 87 --repo wow-look-at-my/grok-build")))

	require.NotEmpty(t, reason, "nothing happened between the two reads")
	assert.Contains(t, reason, "read the state of")
}

func TestRespellingTheQuestionWithAnotherToolIsStillARepeat(t *testing.T) {
	tr := stageTranscript(t,
		callWithID("t1", "gh pr checks 87 --repo wow-look-at-my/grok-build"),
		resultFor("t1", "still running", false),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "mcp__github__pull_request_read",
		`{"owner":"wow-look-at-my","repo":"grok-build","pullNumber":87}`))

	require.NotEmpty(t, reason,
		"the subject is the same pull request, so a different tool is the same call")
	assert.Contains(t, reason, "the same subject is the same call")
}

func TestAWakeEnvelopeIsRecognisedInAToolResult(t *testing.T) {
	tr := stageTranscript(t,
		toolResult(`<wake reason="external-event"><event source="github"/></wake>`),
	)
	recs := parseRecords(tr, "")
	require.Len(t, recs, 1)
	assert.True(t, recs[0].wake, "the envelope arrives escaped inside the result's content")

	// The same record written by an encoder that escapes HTML must read the
	// same, or the guard is blind on half the transcripts it may be handed.
	// json.Marshal escapes HTML by default, so marshaling the whole record IS
	// the escaped spelling.
	escaped, err := json.Marshal(map[string]any{
		"type": "user",
		"message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "content": `<wake reason="external-event"><event source="github"/></wake>`},
		}},
	})
	require.NoError(t, err)
	require.Contains(t, string(escaped), "\\u003c", "this fixture must be the escaped spelling")
	recs = parseRecords(stageTranscript(t, string(escaped)), "")
	require.Len(t, recs, 1)
	assert.True(t, recs[0].wake, "the escaped spelling of the envelope counts too")
}

func TestAWakeEventReopensTheSubject(t *testing.T) {
	tr := stageTranscript(t,
		bashCall("gh pr view 87 --repo wow-look-at-my/grok-build"),
		toolResult(`{"pr":"wow-look-at-my/grok-build#87","state":"open"}`),
		toolResult(`<wake reason="external-event"><event source="github" kind="check_suite.completed"/></wake>`),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr checks 87 --repo wow-look-at-my/grok-build")))

	assert.Empty(t, reason, "an event arriving is exactly the signal worth re-reading on")
}

func TestAUserPromptReopensTheSubject(t *testing.T) {
	tr := stageTranscript(t,
		bashCall("gh pr view 87 --repo wow-look-at-my/grok-build"),
		toolResult(`{"pr":"wow-look-at-my/grok-build#87","state":"open"}`),
		userPrompt("is it green yet?"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr checks 87 --repo wow-look-at-my/grok-build")))

	assert.Empty(t, reason, "the user asking is always a reason to look")
}

func TestReadingALogIsNotReadingAState(t *testing.T) {
	// A log is new information, and refusing it leaves the session unable to
	// diagnose the failure it was woken for.
	tr := stageTranscript(t,
		bashCall("gh wait-ci --sha 9b348b7"),
		toolResult(`{"sha":"9b348b7","conclusion":"failure"}`),
	)
	for _, cmd := range []string{
		"gh wait-ci log 34025531391 --job 101465702969",
		"gh wait-ci grep --sha 9b348b7 'panic'",
		"gh wait-ci annotations 34025531391",
		"gh wait-ci jobs 34025531391",
	} {
		reason := denyReasonOf(t, preToolPayload(t, tr, "Bash", bashInput(cmd)))
		assert.Empty(t, reason, "reading output is how a failure gets fixed: %s", cmd)
	}
}

func TestReadingTheStateAgainIsStillARepeat(t *testing.T) {
	// The negative control for the case above. `watch` answers the question the
	// log does not, and re-asking it learns nothing.
	tr := stageTranscript(t,
		callWithID("t1", "gh wait-ci --sha 9b348b7"),
		resultFor("t1", `{"sha":"9b348b7","conclusion":"failure"}`, false),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci watch --sha 9b348b7")))
	assert.NotEmpty(t, reason, "the state answers the same either time")
}

func TestAMidTurnInterjectionReopensTheSubject(t *testing.T) {
	tr := stageTranscript(t,
		bashCall("gh pr view 87 --repo wow-look-at-my/grok-build"),
		toolResult(`{"pr":"wow-look-at-my/grok-build#87","state":"open"}`),
		toolResult("ok\n\nThe user sent a new message while you were working:\nCI is failed on #87"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr checks 87 --repo wow-look-at-my/grok-build")))

	assert.Empty(t, reason, "a message typed mid-turn is the same message, and the user asking is always a reason to look")
}

func TestAnOrdinaryToolResultStillSettlesTheSubject(t *testing.T) {
	tr := stageTranscript(t,
		bashCall("gh pr view 87 --repo wow-look-at-my/grok-build"),
		toolResult(`{"pr":"wow-look-at-my/grok-build#87","state":"merged"}`),
		toolResult("ok"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr checks 87 --repo wow-look-at-my/grok-build")))

	assert.NotEmpty(t, reason, "without the marker a tool_result is not a message, and a merged pull request stays settled")
}

func TestAPushReopensTheSubject(t *testing.T) {
	tr := stageTranscript(t,
		bashCall("gh pr view 87 --repo wow-look-at-my/grok-build"),
		toolResult(`{"pr":"wow-look-at-my/grok-build#87","state":"open"}`),
		bashCall("git push -u origin claude/fix"),
		toolResult("pushed"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr checks 87 --repo wow-look-at-my/grok-build")))

	assert.Empty(t, reason, "CI on a new head is a new question, not a repeat")
}

// A Bash call's `description` is written for a human. A commit named there is
// not a commit the command asks about.
func TestADescriptionNamingACommitIsNotAReadOfIt(t *testing.T) {
	tr := stageTranscript(t,
		callWithID("t1", "gh wait-ci --sha 58e180d -R wow-look-at-my/go-toolchain"),
		resultFor("t1", "all green", false),
	)

	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash", describedInput(
		"gh wait-ci runs --branch claude/untitled-session-7ejvgb -R wow-look-at-my/go-toolchain",
		"Find the run for 58e180d")))

	assert.Empty(t, reason,
		"the command names no sha; 58e180d appears only in the description, which is prose for the reader")
}

// The same defect in the other direction: a read RECORDED off a description
// makes a genuine later read of that commit look like a repeat.
func TestADescriptionDoesNotMarkASubjectAsAlreadyRead(t *testing.T) {
	tr := stageTranscript(t,
		describedCall("t1", "gh wait-ci runs --branch claude/fix",
			"Look for the run that built 58e180d"),
		resultFor("t1", "listed", false),
	)

	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci --sha 58e180d -R wow-look-at-my/go-toolchain")))

	assert.Empty(t, reason, "nothing has read that commit's state yet")
}

func TestAListingNamingSeveralPullRequestsSettlesNone(t *testing.T) {
	tr := stageTranscript(t,
		toolResult(`[{"pr":"wow-look-at-my/grok-build#87","state":"merged"},`+
			`{"pr":"wow-look-at-my/grok-build#88","state":"open"}]`),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr view 88 --repo wow-look-at-my/grok-build")))

	assert.Empty(t, reason,
		"a verdict that cannot be attributed to one pull request must settle neither")
}

func TestACallThatIsNotAStatusReadIsNeverRefused(t *testing.T) {
	tr := stageTranscript(t,
		toolResult(`{"outcome":"merged","pr":"wow-look-at-my/grok-build#87"}`),
	)
	for _, c := range []struct{ tool, input string }{
		{"Read", `{"file_path":"/repo/wow-look-at-my/grok-build#87.md"}`},
		{"Bash", bashInput("git log --oneline wow-look-at-my/grok-build#87")},
		{"Bash", bashInput("grep -r 'gh pr view' docs/")},
	} {
		assert.Empty(t, denyReasonOf(t, preToolPayload(t, tr, c.tool, c.input)),
			"%s must run: it is not asking after a status", c.tool)
	}
}

func TestACommandThatOnlyMentionsAStatusReadIsNotOne(t *testing.T) {
	assert.False(t, namesAStatusCommand("grep -rn 'gh wait-ci' claude_snippets/"))
	assert.False(t, namesAStatusCommand("git commit -m 'document gh pr view usage'"))
	assert.True(t, namesAStatusCommand("gh wait-ci --sha abc1234"))
	assert.True(t, namesAStatusCommand("cd /repo && gh pr checks 12"))
	assert.True(t, namesAStatusCommand("echo hi; gh run list"))
}

func TestSubjectsAreSpelledTheSameAcrossTools(t *testing.T) {
	fromBash := subjectsIn(strings.ToLower(`gh pr view 87 --repo wow-look-at-my/grok-build`))
	fromMCP := subjectsIn(strings.ToLower(`{"owner":"wow-look-at-my","repo":"grok-build","pullNumber":87}`))
	fromSlug := subjectsIn(strings.ToLower(`wow-look-at-my/grok-build#87`))

	assert.Contains(t, fromBash, "pr:wow-look-at-my/grok-build#87")
	assert.Contains(t, fromMCP, "pr:wow-look-at-my/grok-build#87")
	assert.Contains(t, fromSlug, "pr:wow-look-at-my/grok-build#87")
}

func TestAHexWordIsNotACommitSubject(t *testing.T) {
	assert.NotContains(t, subjectsIn("the defaced banner"), "sha:defaced")
	assert.Contains(t, subjectsIn("commit c274ad3"), "sha:c274ad3")
}

func TestEveryFailurePathAllowsTheCall(t *testing.T) {
	tr := stageTranscript(t, toolResult(`{"outcome":"merged","pr":"wow-look-at-my/grok-build#87"}`))

	t.Run("unparseable payload", func(t *testing.T) {
		assert.Equal(t, allow(), Run(strings.NewReader("{not json")))
	})
	t.Run("another event", func(t *testing.T) {
		assert.Equal(t, allow(), Run(strings.NewReader(
			`{"hook_event_name":"PostToolUse","tool_name":"Bash"}`)))
	})
	t.Run("no tool name", func(t *testing.T) {
		assert.Equal(t, allow(), Run(strings.NewReader(encodeLine(map[string]any{
			"hook_event_name": "PreToolUse", "transcript_path": tr,
		}))))
	})
	t.Run("missing transcript", func(t *testing.T) {
		assert.Empty(t, denyReasonOf(t, preToolPayload(t, "/nonexistent/transcript.jsonl",
			"Bash", bashInput("gh pr view 87 --repo wow-look-at-my/grok-build"))))
	})
	t.Run("a status read naming no subject", func(t *testing.T) {
		assert.Empty(t, denyReasonOf(t, preToolPayload(t, tr, "Bash", bashInput("gh run list"))))
	})
}

func TestTheStopHalfStillWorks(t *testing.T) {
	assert.Equal(t, allow(), Run(strings.NewReader(
		`{"hook_event_name":"Stop","transcript_path":"/nonexistent"}`)),
		"adding a second event must not change the Stop half's fail-open path")
}
