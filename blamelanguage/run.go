// run.go is this package's hook entry point: the MessageDisplay payload in,
// the displayContent envelope out.
//
// MessageDisplay, not Stop: a Stop refusal on wording only buys a retype. The
// message arrives in flushes, so the text is accumulated in a per-message file
package blamelanguage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/wow-look-at-my/go-containers/set"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Input is the part of the MessageDisplay payload this reads.
type Input struct {
	HookEventName string `json:"hook_event_name"`
	MessageID     string `json:"message_id"`
	Final         bool   `json:"final"`
	Delta         string `json:"delta"`
}

// Result is what the CLI prints and exits with.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// named caps the phrases the annotation quotes.
const named = 3

// disabled reports the escape hatch. Several spellings: testing a single value
// breaks an opt-out silently.
func disabled() bool {
	switch os.Getenv("CC_NO_BLAME_LANGUAGE") {
	case "0", "false", "no", "off":
		return true
	}
	return false
}

// Run judges the finished message and annotates it. Every failure path prints
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
	if in.MessageID == "" {
		return Result{}
	}

	message, ok := accumulate(in)
	if !ok || message == "" {
		return Result{}
	}
	line := annotation(Check(message))
	if line == "" {
		return Result{}
	}
	// displayContent REPLACES the delta, so send the delta and not the whole
	// accumulated message: that renders the text again.
	return Result{Stdout: envelope(in.Delta + line)}
}

// accumulate appends this flush and answers the whole message at the end. An
// earlier flush answers not-ready rather than empty.
func accumulate(in Input) (string, bool) {
	p := statePath(in.MessageID)
	if p == "" {
		return "", false
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", false
	}
	_, writeErr := f.WriteString(in.Delta)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return "", false
	}
	if !in.Final {
		return "", false
	}
	data, err := os.ReadFile(p)
	os.Remove(p)
	// A message with no final flush strands its file. Swept here, not per flush.
	sweep()
	if err != nil {
		return "", false
	}
	return string(data), true
}

// annotation renders the line the reader sees, or nothing when the message is
// clean. It names the phrases the rule found, which is what makes the note
// actionable rather than a scolding.
func annotation(hits []Hit) string {
	seen := set.New[string]()
	var phrases []string
	for _, hit := range hits {
		phrase := hit.Phrase
		if phrase == "" {
			phrase = hit.Sentence
		}
		if phrase == "" || seen.Contains(phrase) {
			continue
		}
		seen.Add(phrase)
		phrases = append(phrases, `"`+phrase+`"`)
	}
	if len(phrases) == 0 {
		return ""
	}
	more := ""
	if len(phrases) > named {
		phrases = phrases[:named]
		more = ", and more"
	}
	return "\n\n> **no-blame-language** -- " + strings.Join(phrases, ", ") + more +
		". Every repository here was written by the same hand, so there is no " +
		"other author to hand this to. Own it and say what you fixed."
}

// envelope wraps the annotated text in the only field the event carries.
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

// statePath keys the accumulated text by message, so parallel messages never
// mix and neither do parallel sessions.
func statePath(messageID string) string {
	sum := sha256.Sum256([]byte(messageID))
	return filepath.Join(os.TempDir(), statePrefix+hex.EncodeToString(sum[:])[:16])
}

const statePrefix = "slopfix-blame-"

// sweep collects what a session that died mid-message left behind, so the temp
// directory cannot grow without bound. A file still in use is younger than the
// cutoff, so a sweep never takes it.
func sweep() {
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), statePrefix+"*"))
	if err != nil {
		return
	}
	for _, p := range matches {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > time.Hour {
			os.Remove(p)
		}
	}
}
