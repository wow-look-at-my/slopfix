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
	path := filepath.Join(t.TempDir(), "t.jsonl")
	lines := `{"type":"user","message":{"content":"go on"}}
{"type":"assistant","message":{"content":[{"type":"text","text":"earlier"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":` + quote(t, punt) + `}]}}
`
	require.NoError(t, os.WriteFile(path, []byte(lines), 0o600))

	res := run(t, payload(t, map[string]any{"transcript_path": path}))
	assert.Equal(t, 2, res.Code)
}

// A transcript entry can carry its text as a plain string.
func TestATranscriptStringContentIsRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	line := `{"role":"assistant","content":` + quote(t, punt) + "}\n"
	require.NoError(t, os.WriteFile(path, []byte(line), 0o600))

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

func quote(t *testing.T, s string) string {
	t.Helper()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	return string(data)
}
