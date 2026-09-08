// run.go is this package's entry point: the MessageDisplay payload in, the
// displayContent envelope out.
//
// The message arrives in flushes, and a fenced block can span them, so the
// fence state is carried in a per-message file under the temp directory. A lost
// state file costs the fence exemption on later flushes of that message,
// never a wrong rewrite of another.
package linkrefs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// Input is the part of the MessageDisplay payload this reads.
type Input struct {
	HookEventName string `json:"hook_event_name"`
	MessageID     string `json:"message_id"`
	Index         int    `json:"index"`
	Final         bool   `json:"final"`
	Delta         string `json:"delta"`
}

// Result is what the CLI prints and exits with.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// disabled reports the escape hatch: a guard that eats output is worse than none.
func disabled() bool { return os.Getenv("CC_LINK_ALL_REFS") == "0" }

// Run rewrites the flush it is given and returns the envelope to print.
// Every failure path prints nothing, which leaves the original text on screen.
func Run(r io.Reader) Result {
	if disabled() {
		return Result{}
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return Result{}
	}
	var in Input
	if json.Unmarshal(data, &in) != nil {
		return Result{}
	}
	if in.HookEventName != "" && in.HookEventName != "MessageDisplay" {
		return Result{}
	}
	if in.Delta == "" {
		clearFence(in.MessageID, in.Final)
		return Result{}
	}

	inFence := readFence(in.MessageID)
	out, changed := RewriteDelta(in.Delta, inFence, &GitResolver{})
	writeFence(in.MessageID, in.Final, inFence != EndsInsideFence(in.Delta))
	if !changed {
		return Result{}
	}
	return Result{Stdout: envelope(out)}
}

// envelope wraps the rewritten text in the only field the event carries.
func envelope(text string) string {
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":  "MessageDisplay",
			"displayContent": text,
		},
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(out)
}

// fencePath keys the state by message, so parallel messages never mix.
func fencePath(messageID string) string {
	if messageID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(messageID))
	return filepath.Join(os.TempDir(), "slopfix-linkrefs-"+hex.EncodeToString(sum[:])[:16])
}

func readFence(messageID string) bool {
	p := fencePath(messageID)
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

func writeFence(messageID string, final, inside bool) {
	p := fencePath(messageID)
	if p == "" {
		return
	}
	if final {
		os.Remove(p)
		return
	}
	if inside {
		os.WriteFile(p, nil, 0o600)
		return
	}
	os.Remove(p)
}

func clearFence(messageID string, final bool) {
	if !final {
		return
	}
	if p := fencePath(messageID); p != "" {
		os.Remove(p)
	}
}
