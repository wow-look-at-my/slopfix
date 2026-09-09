package busypoll

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMain gives every other test in this package a remote session, which is
// the environment those rules apply to. The local case is asserted here.
func TestMain(m *testing.M) {
	os.Setenv("CLAUDE_CODE_REMOTE", "true")
	os.Exit(m.Run())
}

func TestRemoteSessionReadsEitherVariable(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		id     string
		want   bool
	}{
		{"web session", "true", "cse_01", true},
		{"flag alone", "1", "", true},
		{"id alone", "", "cse_01", true},
		{"terminal", "", "", false},
		{"flag turned off", "false", "", false},
		{"flag set to zero", "0", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_REMOTE", c.remote)
			t.Setenv("CLAUDE_CODE_REMOTE_SESSION_ID", c.id)
			assert.Equal(t, c.want, remoteSession())
		})
	}
}

func TestStopIsAllowedInALocalSession(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REMOTE", "")
	t.Setenv("CLAUDE_CODE_REMOTE_SESSION_ID", "")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	path := closePoll(t, 6, base, "gh pr view 186 --json state,mergedAt")
	res := Run(strings.NewReader(stopPayload(t, path, false)))
	assert.Equal(t, 0, res.Code, "no event reaches a local session, so the re-check is the only signal")
}

func TestRepeatedReadIsAllowedInALocalSession(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REMOTE", "")
	t.Setenv("CLAUDE_CODE_REMOTE_SESSION_ID", "")
	path := stageTranscript(t,
		bashCall("gh pr view 186 --json state"),
		toolResult(`{"state":"OPEN"}`),
	)
	res := Run(strings.NewReader(preToolPayload(t, path, "Bash",
		`{"command":"gh pr view 186 --json state"}`)))
	assert.Empty(t, res.Stdout, "a local session learns of a state change by asking again")
}

func TestTerminalSubjectIsRefusedInALocalSession(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REMOTE", "")
	t.Setenv("CLAUDE_CODE_REMOTE_SESSION_ID", "")
	path := stageTranscript(t,
		bashCall("gh pr view 186 --json state"),
		toolResult(`{"number":186,"state":"MERGED","merged":true}`),
	)
	res := Run(strings.NewReader(preToolPayload(t, path, "Bash",
		`{"command":"gh pr view 186 --json state"}`)))
	assert.Contains(t, res.Stdout, "already reached a state it cannot leave",
		"a merged pull request does not un-merge on a laptop either")
}
