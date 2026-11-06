package busypoll

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	// same. Otherwise, the guard is blind on half the transcripts it may be
	// handed. json.Marshal escapes HTML by default, so marshaling the whole
	// record IS the escaped spelling.
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
	// A log is new information.
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
