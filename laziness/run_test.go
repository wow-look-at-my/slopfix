package laziness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// punt is a closing message this rule exists to refuse.
const punt = "I found an off-by-one in the retry loop and left it alone."

// clean is the control: a message that reports a fix rather than a defect.
const clean = "Fixed the off-by-one in the retry loop and pushed it."

func payload(t *testing.T, fields map[string]any) string {
	t.Helper()
	if _, ok := fields["hook_event_name"]; !ok {
		fields["hook_event_name"] = "Stop"
	}
	data, err := json.Marshal(fields)
	require.NoError(t, err)
	return string(data)
}

func run(t *testing.T, body string) Result {
	t.Helper()
	return Run(strings.NewReader(body))
}

func TestAPuntIsRefused(t *testing.T) {
	res := run(t, payload(t, map[string]any{"last_assistant_message": punt}))
	assert.Equal(t, 2, res.Code)
	assert.NotEmpty(t, res.Stderr)
}

// The control that proves the case above can fail.
func TestAMessageThatFixedItIsAllowed(t *testing.T) {
	assert.Equal(t, Result{}, run(t, payload(t, map[string]any{"last_assistant_message": clean})))
}

// The payload does not always carry the message. Without the fallback such a
// turn ends unjudged, which reads as a guard that is off.
func TestTheTranscriptIsReadWhenTheFieldIsAbsent(t *testing.T) {
	path := transcript(t,
		map[string]any{"type": "user", "message": map[string]any{"content": "go on"}},
		assistantParts("earlier"),
		assistantParts(punt),
	)

	res := run(t, payload(t, map[string]any{"transcript_path": path}))
	assert.Equal(t, 2, res.Code)
}

// assistantParts is the shape whose content is a list of typed parts.
func assistantParts(text string) map[string]any {
	return map[string]any{
		"type":    "assistant",
		"message": map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}},
	}
}

// transcript writes the entries as JSONL and answers the path.
func transcript(t *testing.T, entries ...map[string]any) string {
	t.Helper()
	var b strings.Builder
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		require.NoError(t, err)
		b.Write(data)
		b.WriteByte('\n')
	}
	path := filepath.Join(t.TempDir(), "t.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(b.String()), 0o600))
	return path
}

// A transcript entry can carry its text as a plain string.
func TestATranscriptStringContentIsRead(t *testing.T) {
	path := transcript(t, map[string]any{"role": "assistant", "content": punt})

	assert.Equal(t, 2, run(t, payload(t, map[string]any{"transcript_path": path})).Code)
}

// The refusal fires at most per turn: a message that cannot be rewritten would
// otherwise trip this forever.
func TestAnAlreadyActiveStopIsAllowed(t *testing.T) {
	body := payload(t, map[string]any{"last_assistant_message": punt, "stop_hook_active": true})
	assert.Equal(t, Result{}, run(t, body))
}

func TestAnotherEventIsSilent(t *testing.T) {
	body := payload(t, map[string]any{"hook_event_name": "SessionEnd", "last_assistant_message": punt})
	assert.Equal(t, Result{}, run(t, body))
}

// Every failure path allows the stop: a guard that wedges a session is worse
// than no guard.
func TestEveryUnreadablePayloadIsAllowed(t *testing.T) {
	for _, body := range []string{"{ not json", "", "{}", `{"transcript_path":"/nowhere/at/all"}`} {
		assert.Equal(t, Result{}, run(t, body), body)
	}
}
