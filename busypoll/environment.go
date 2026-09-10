// environment.go decides whether this session hears about the world without
// asking for it.
//
// A remote session gets wake events: a webhook for a review or a check run, a
// notification, a scheduled trigger. It can stop and be woken, so a re-read
// that beats the event only spends tokens. A local session gets none of that.
// There, a check nobody makes is a check that never happens, and a repeated
// read is the only way to learn that CI finished.
//
// So the two rules that tell the model to wait apply to a remote session
// only. The terminal rule applies everywhere: a merged pull request does not
// un-merge on a laptop either.
package busypoll

import (
	"os"
	"strings"
)

// remoteSession reports whether wake events reach this session. Claude Code on
// the web sets both variables. A terminal session sets neither.
func remoteSession() bool {
	if os.Getenv("CLAUDE_CODE_REMOTE_SESSION_ID") != "" {
		return true
	}
	return truthy(os.Getenv("CLAUDE_CODE_REMOTE"))
}

// truthy reads a flag variable. Anything the shell spells as off is off, and
// an unset variable is off.
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no", "off":
		return false
	}
	return true
}
