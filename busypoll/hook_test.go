package busypoll

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closePoll writes n turns, each a Bash call with the same command, seconds
// apart -- the shape this rule exists to catch.
func closePoll(t *testing.T, n int, base time.Time, cmd string) string {
	t.Helper()
	var lines []string
	for i := range n {
		start := base.Add(time.Duration(i) * 20 * time.Second)
		lines = append(lines,
			rec(t, "user", "user", textBlock("Stop hook feedback: still waiting"), start),
			rec(t, "assistant", "assistant", toolUseBlock("Bash", map[string]any{"command": cmd}), start.Add(time.Second)),
			rec(t, "user", "user", toolResultBlock(), start.Add(2*time.Second)),
			rec(t, "assistant", "assistant", textBlock("still open, holding"), start.Add(3*time.Second)),
		)
	}
	return writeLines(t, lines)
}

func stopPayload(t *testing.T, path string, active bool) string {
	t.Helper()
	in, err := json.Marshal(Input{
		HookEventName:  "Stop",
		TranscriptPath: path,
		StopHookActive: active,
	})
	require.NoError(t, err)
	return string(in)
}

func TestStopIsRefusedAfterFourCloselySpacedIdenticalTurns(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	path := closePoll(t, 4, base, "gh pr view 186 --json state,mergedAt")
	res := Run(strings.NewReader(stopPayload(t, path, false)))
	assert.Equal(t, 2, res.Code)
	assert.Contains(t, res.Stderr, "gh pr view 186")
	assert.Contains(t, res.Stderr, "busy-poll")
	assert.Contains(t, res.Stderr, "ScheduleWakeup")
}

func TestStopIsAllowedWithFewerThanTheThreshold(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	path := closePoll(t, 3, base, "gh pr view 186 --json state,mergedAt")
	res := Run(strings.NewReader(stopPayload(t, path, false)))
	assert.Equal(t, 0, res.Code, "three quick re-checks can be a person iterating by hand")
}

func TestStopIsAllowedWhenTurnsAreProperlyPaced(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var lines []string
	for i := range 5 {
		start := base.Add(time.Duration(i) * 20 * time.Minute)
		lines = append(lines,
			rec(t, "user", "user", textBlock("check-in fired"), start),
			rec(t, "assistant", "assistant", toolUseBlock("Bash", map[string]any{"command": "gh pr checks 42"}), start.Add(time.Second)),
			rec(t, "user", "user", toolResultBlock(), start.Add(2*time.Second)),
			rec(t, "assistant", "assistant", textBlock("still pending, rearming"), start.Add(3*time.Second)),
		)
	}
	res := Run(strings.NewReader(stopPayload(t, writeLines(t, lines), false)))
	assert.Equal(t, 0, res.Code, "a properly spaced watch loop is not the pattern this hook refuses")
}

func TestStopIsAllowedWhenTheCurrentTurnBreaksThePattern(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	path := closePoll(t, 4, base, "gh pr view 186 --json state,mergedAt")

	// The model heeded an earlier refusal and replies with no tool call.
	lines := []string{}
	data, err := readTail(path, transcriptTailBytes)
	require.NoError(t, err)
	for _, l := range data {
		if len(l) > 0 {
			lines = append(lines, string(l))
		}
	}
	lastStart := base.Add(4 * 20 * time.Second)
	lines = append(lines,
		rec(t, "user", "user", textBlock("Stop hook feedback: still waiting"), lastStart),
		rec(t, "assistant", "assistant", textBlock("Holding, no new check performed."), lastStart.Add(time.Second)),
	)
	res := Run(strings.NewReader(stopPayload(t, writeLines(t, lines), false)))
	assert.Equal(t, 0, res.Code, "the current turn made no tool call, so nothing is being repeated right now")
}

func TestStopIsAllowedWhenTheCallsDifferEachTime(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var lines []string
	for i := range 5 {
		start := base.Add(time.Duration(i) * 20 * time.Second)
		lines = append(lines,
			rec(t, "user", "user", textBlock("fix and retest"), start),
			rec(t, "assistant", "assistant", toolUseBlock("Edit", map[string]any{"file_path": "x.go", "new_string": "v" + string(rune('0'+i))}), start.Add(time.Second)),
			rec(t, "assistant", "assistant", toolUseBlock("Bash", map[string]any{"command": "go test ./..."}), start.Add(2*time.Second)),
			rec(t, "user", "user", toolResultBlock(), start.Add(3*time.Second)),
		)
	}
	res := Run(strings.NewReader(stopPayload(t, writeLines(t, lines), false)))
	assert.Equal(t, 0, res.Code, "each turn's edit differs, so the whole turn is never byte-identical to the last")
}

func TestRefusalEscalatesOnRepeat(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	path := closePoll(t, 4, base, "gh pr view 186 --json state,mergedAt")
	first := Run(strings.NewReader(stopPayload(t, path, false)))
	second := Run(strings.NewReader(stopPayload(t, path, true)))
	assert.Equal(t, 2, first.Code)
	assert.Equal(t, 2, second.Code)
	assert.NotContains(t, first.Stderr, "not the first refusal")
	assert.Contains(t, second.Stderr, "not the first refusal")
}

func TestUnreadableInputAllowsTheStop(t *testing.T) {
	assert.Equal(t, 0, Run(strings.NewReader("not json")).Code)
	assert.Equal(t, 0, Run(strings.NewReader("")).Code)
}

func TestAMissingTranscriptAllowsTheStop(t *testing.T) {
	assert.Equal(t, 0, Run(strings.NewReader(stopPayload(t, filepath.Join(t.TempDir(), "absent.jsonl"), false))).Code)
	assert.Equal(t, 0, Run(strings.NewReader(stopPayload(t, "", false))).Code)
}

func TestAnotherEventIsIgnored(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	path := closePoll(t, 4, base, "gh pr view 186 --json state,mergedAt")
	in, err := json.Marshal(map[string]string{"hook_event_name": "SessionStart", "transcript_path": path})
	require.NoError(t, err)
	assert.Equal(t, 0, Run(strings.NewReader(string(in))).Code)
}

func TestAPayloadWithNoEventNameIsTreatedAsStop(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	path := closePoll(t, 4, base, "gh pr view 186 --json state,mergedAt")
	in, err := json.Marshal(map[string]string{"transcript_path": path})
	require.NoError(t, err)
	assert.Equal(t, 2, Run(strings.NewReader(string(in))).Code)
}

func TestAllowIsSilent(t *testing.T) {
	res := allow()
	assert.Equal(t, 0, res.Code)
	assert.Empty(t, res.Stderr)
}
