package busypoll

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// justNow is a timestamp secondsAgo seconds old. The guard ages evidence out
// against the clock, so a staged record has to sit near it.
func justNow(secondsAgo int) string {
	return time.Now().UTC().Add(-time.Duration(secondsAgo) * time.Second).Format(time.RFC3339)
}

// stageTranscript writes lines as a JSONL transcript and returns its path.
func stageTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	return path
}

// bashCall is an assistant record running a Bash command.
func bashCall(command string) string {
	return bashCallAt(justNow(3), command)
}

// bashCallAt is bashCall written at the timestamp ts.
func bashCallAt(ts, command string) string {
	input, _ := json.Marshal(map[string]string{"command": command})
	return assistantCallAt(ts, "Bash", string(input))
}

// assistantCallAt is an assistant record making a tool call at the timestamp ts.
func assistantCallAt(ts, name, input string) string {
	return encodeLine(map[string]any{
		"type":      "assistant",
		"timestamp": ts,
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
	return toolResultAt(justNow(2), text)
}

// toolResultAt is toolResult written at the timestamp ts.
func toolResultAt(ts, text string) string {
	return encodeLine(map[string]any{
		"type":      "user",
		"timestamp": ts,
		"message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "content": text},
		}},
	})
}

// userPrompt is a genuine new prompt, which re-opens every subject.
func userPrompt(text string) string {
	return encodeLine(map[string]any{
		"type":      "user",
		"timestamp": justNow(1),
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
	return callWithIDAt(justNow(3), id, command)
}

// callWithIDAt is callWithID written at the timestamp ts.
func callWithIDAt(ts, id, command string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant", "timestamp": ts,
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
		"type": "assistant", "timestamp": justNow(3),
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
	return resultForAt(justNow(2), id, text, failed)
}

// resultForAt is resultFor written at the timestamp ts.
func resultForAt(ts, id, text string, failed bool) string {
	block := map[string]any{"type": "tool_result", "tool_use_id": id, "content": text}
	if failed {
		block["is_error"] = true
	}
	b, _ := json.Marshal(map[string]any{
		"type": "user", "timestamp": ts,
		"message": map[string]any{"role": "user", "content": []any{block}},
	})
	return string(b)
}

// callIn and resultIn are stamped with the session that wrote the record, and
// with the sidechain flag a subagent's records carry.
func callIn(id, command, session string, sidechain bool) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant", "timestamp": justNow(3),
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
		"type": "user", "timestamp": justNow(2),
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
// had died on an unknown flag.
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

func TestAPendingBuildCountIsNotAVerdict(t *testing.T) {
	const sha = "31b41ca7781c48fa37ba1e34b0e618e995bb0e9a"
	tr := stageTranscript(t,
		bashCall("gh wait-ci checks --sha "+sha),
		toolResult("31b41ca  (rollup: pending)\nall-builds  pending  0/1 builds passed - waiting on: build"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci --sha "+sha)))

	assert.Empty(t, reason, "nothing has passed yet, so there is no verdict to re-read")
}

// A read often shows more than a single count: a run listing, a comparison with
// an earlier commit, a table of pull requests. The commit this record names is
// the thing still building, and the full count belongs to something else.
func TestAFullCountBesideAPartialOneIsNotAVerdict(t *testing.T) {
	const sha = "31b41ca7781c48fa37ba1e34b0e618e995bb0e9a"
	tr := stageTranscript(t,
		bashCall("gh wait-ci checks --sha "+sha),
		toolResult("the run before this one reported 15/15 builds passed\n"+
			"31b41ca  (rollup: pending)\nall-builds  pending  6/15 builds passed - waiting on: suite"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci --sha "+sha)))

	assert.Empty(t, reason, "this commit is still building, whatever the earlier one did")
}

func TestAFullBuildCountIsAVerdict(t *testing.T) {
	const sha = "31b41ca7781c48fa37ba1e34b0e618e995bb0e9a"
	tr := stageTranscript(t,
		bashCall("gh wait-ci checks --sha "+sha),
		toolResult("Commit: "+sha+"\nall-builds  success  3/3 builds passed"),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci --sha "+sha)))

	require.NotEmpty(t, reason, "every build passed, so the commit is settled")
	assert.Contains(t, reason, "31b41ca")
}

// The refusal has to lapse, or a single early observation outlives the thing
// it observed.
func TestAGreenCommitIsReadableAgainOnceTheVerdictIsStale(t *testing.T) {
	const sha = "c274ad3c1a9c7bc156d706dc6062b2ab298417c0"
	tr := stageTranscript(t,
		bashCallAt(justNow(1810), "gh wait-ci --sha "+sha),
		toolResultAt(justNow(1800), "CI PASSED\nCommit: "+sha),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci checks --sha "+sha)))

	assert.Empty(t, reason, "half an hour later the verdict is not the answer any more")
}

func TestAGreenCommitIsStillShutInsideTheWindow(t *testing.T) {
	const sha = "c274ad3c1a9c7bc156d706dc6062b2ab298417c0"
	tr := stageTranscript(t,
		bashCallAt(justNow(120), "gh wait-ci --sha "+sha),
		toolResultAt(justNow(115), "CI PASSED\nCommit: "+sha),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci checks --sha "+sha)))

	require.NotEmpty(t, reason, "two minutes later nothing can have changed")
	assert.Contains(t, reason, "5 minutes", "the refusal must say when it lapses")
}

func TestARepeatedReadIsAllowedAgainOnceTheReadIsStale(t *testing.T) {
	tr := stageTranscript(t,
		callWithIDAt(justNow(1810), "t1", "gh pr view 87 --repo wow-look-at-my/grok-build"),
		resultForAt(justNow(1800), "t1", `{"pr":"wow-look-at-my/grok-build#87","state":"open"}`, false),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh pr checks 87 --repo wow-look-at-my/grok-build")))

	assert.Empty(t, reason, "a wait this long is a real wait, not a busy-poll")
}

// The window is a single knob for both halves: what counts as no real wait
// having happened is what counts as evidence still being current.
func TestTheWindowFollowsTheGapSetting(t *testing.T) {
	t.Setenv("NO_BUSY_POLL_MAX_GAP_SECONDS", "3600")
	const sha = "c274ad3c1a9c7bc156d706dc6062b2ab298417c0"
	tr := stageTranscript(t,
		bashCallAt(justNow(1810), "gh wait-ci --sha "+sha),
		toolResultAt(justNow(1800), "CI PASSED\nCommit: "+sha),
	)
	reason := denyReasonOf(t, preToolPayload(t, tr, "Bash",
		bashInput("gh wait-ci checks --sha "+sha)))

	require.NotEmpty(t, reason, "an hour-long window still covers a half-hour-old verdict")
	assert.Contains(t, reason, "60 minutes")
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
