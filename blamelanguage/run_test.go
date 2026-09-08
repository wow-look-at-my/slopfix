package blamelanguage

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deflecting is a closing message this rule exists to mark.
const deflecting = "The suite is red, but the failure is pre-existing."

// owned is the control: the same report, without the deflection.
const owned = "The suite is red. I broke it in the retry loop and I am fixing it."

func flush(t *testing.T, id, delta string, final bool) Result {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"hook_event_name": "MessageDisplay",
		"message_id":      id,
		"final":           final,
		"delta":           delta,
	})
	require.NoError(t, err)
	return Run(strings.NewReader(string(data)))
}

// displayContent carries the delta plus the note. It REPLACES the delta, so a
// whole accumulated message here would render the text again.
func shown(t *testing.T, res Result) string {
	t.Helper()
	var out struct {
		HookSpecificOutput struct {
			HookEventName  string `json:"hookEventName"`
			DisplayContent string `json:"displayContent"`
		} `json:"hookSpecificOutput"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out))
	assert.Equal(t, "MessageDisplay", out.HookSpecificOutput.HookEventName)
	return out.HookSpecificOutput.DisplayContent
}

func TestADeflectingMessageIsMarked(t *testing.T) {
	res := flush(t, t.Name(), deflecting, true)
	require.NotEmpty(t, res.Stdout)
	text := shown(t, res)
	assert.Contains(t, text, deflecting)
	assert.Contains(t, text, "no-blame-language")
	assert.Contains(t, text, "pre-existing")
}

// The control that proves the case above can fail.
func TestAnOwnedReportIsLeftAlone(t *testing.T) {
	assert.Equal(t, Result{}, flush(t, t.Name(), owned, true))
}

// A phrase can span a flush boundary. Judging a flush by itself misses it.
func TestAPhraseSplitAcrossFlushesIsStillFound(t *testing.T) {
	head, tail, ok := strings.Cut(deflecting, "pre-")
	require.True(t, ok)

	assert.Equal(t, Result{}, flush(t, t.Name(), head, false), "a flush before the end says nothing")
	res := flush(t, t.Name(), "pre-"+tail, true)
	require.NotEmpty(t, res.Stdout)
	assert.Contains(t, shown(t, res), "no-blame-language")
}

// The note carries only the delta, never the text of the earlier flushes.
func TestTheNoteRidesTheFinalDeltaOnly(t *testing.T) {
	head, tail, ok := strings.Cut(deflecting, "pre-")
	require.True(t, ok)
	flush(t, t.Name(), head, false)

	text := shown(t, flush(t, t.Name(), "pre-"+tail, true))
	assert.NotContains(t, text, head, "the earlier flush is already on screen")
}

// A message keys its own state, so parallel messages never mix.
func TestAnotherMessageDoesNotInherit(t *testing.T) {
	flush(t, t.Name()+"-a", deflecting, false)
	assert.Equal(t, Result{}, flush(t, t.Name()+"-b", owned, true))
}

func TestTheOptOutIsHonoured(t *testing.T) {
	for _, value := range []string{"0", "false", "no", "off"} {
		t.Setenv("CC_NO_BLAME_LANGUAGE", value)
		assert.Equal(t, Result{}, flush(t, t.Name()+value, deflecting, true), value)
	}
}

func TestAnotherEventIsSilent(t *testing.T) {
	body := `{"hook_event_name":"Stop","message_id":"m","final":true,"delta":"` + deflecting + `"}`
	assert.Equal(t, Result{}, Run(strings.NewReader(body)))
}

// Every failure path prints nothing, which leaves the text on screen.
func TestEveryUnreadablePayloadIsSilent(t *testing.T) {
	for _, body := range []string{"{ not json", "", "{}", `{"final":true,"delta":"x"}`} {
		assert.Equal(t, Result{}, Run(strings.NewReader(body)), body)
	}
}
