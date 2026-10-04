// pretool.go is the deny half: a status read is refused BEFORE it runs, so
// the wasted call costs nothing rather than costing a round trip and then
// earning a note about it at Stop.
//
// The rules here answer the same question -- can this call learn anything? A
// subject that has already reached a terminal state cannot teach it anything
// ever again.
package busypoll

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wow-look-at-my/go-containers/set"
)

// worldChangers are the calls after which re-reading a status is legitimate.
var worldChangers = []string{
	"git push",
	"git commit",
	"git merge",
	"gh wait-ci dispatch",
	"gh wait-ci rerun",
}

// verdict is the refusal, or empty when the call may run.
type verdict struct {
	deny   bool
	reason string
}

// judgeCall decides a PreToolUse call against the transcript behind it.
func judgeCall(c toolCall, recs []record) verdict {
	if !isStatusRead(c) {
		return verdict{}
	}
	subs := subjectsIn(strings.ToLower(callText(c)))
	if len(subs) == 0 {
		return verdict{}
	}

	// Evidence ages out. See staleness.go.
	recs = recentRecords(recs)

	terminal := terminalSubjects(recs)
	for _, s := range subs {
		if terminal[s] {
			return verdict{deny: true, reason: terminalReason(s)}
		}
	}

	// A local session is never woken, so a re-read there is the only way to
	// learn that the answer moved.
	if !remoteSession() {
		return verdict{}
	}

	already := readSinceLastSignal(recs)
	for _, s := range subs {
		if already[s] {
			return verdict{deny: true, reason: repeatReason(s)}
		}
	}
	return verdict{}
}

// readSinceLastSignal returns the subjects already read since the last thing that could have changed an answer.
func readSinceLastSignal(recs []record) map[string]bool {
	start := 0
	for i, r := range recs {
		if r.newPrompt || r.wake {
			start = i
			continue
		}
		for _, c := range r.calls {
			if c.name == "Bash" && containsAny(strings.ToLower(commandOf(c.input)), worldChangers) {
				start = i
			}
		}
	}

	// A result arrives after the call it answers, so results are collected before the reads are judged against them.
	failed := set.New[string]()
	answered := set.New[string]()
	for _, r := range recs[start:] {
		for _, id := range r.failed {
			failed.Add(id)
		}
		for _, id := range r.answered {
			answered.Add(id)
		}
	}

	out := map[string]bool{}
	for _, r := range recs[start:] {
		for _, c := range r.calls {
			if !isStatusRead(c) || !answered.Contains(c.id) || failed.Contains(c.id) {
				continue
			}
			for _, s := range subjectsIn(strings.ToLower(callText(c))) {
				out[s] = true
			}
		}
	}
	return out
}

// terminalText is what the model is told about a settled subject. It is a
// document with a hole in it rather than a run of writes, so the wording
// reads as the reader will see it.
const terminalText = `Blocked: %s is settled and this session watched it settle.
The state is in your transcript. A push makes a new commit, which is a new question.

This refusal lapses %s after the verdict, in case the verdict covered only part
of the answer.`

// repeatText names the ways out, because a refusal that only says "do not"
// costs a round trip while the model guesses at what would satisfy it.
const repeatText = `Blocked: you read the state of %s already, and nothing has
happened since -- no user message, no wake event, no push of your own. Another
tool asking after the same subject is the same call.

This unblocks itself the moment anything real happens, and in any case %s
after the read you already have.`

func terminalReason(subject string) string {
	return fmt.Sprintf(terminalText, describe(subject), spellGap(maxGap()))
}

func repeatReason(subject string) string {
	return fmt.Sprintf(repeatText, describe(subject), spellGap(maxGap()))
}

// spellGap writes a window the way the refusal reads it out.
func spellGap(d time.Duration) string {
	if d%time.Minute == 0 {
		return plural(int(d/time.Minute), "minute")
	}
	return plural(int(d/time.Second), "second")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// describe renders a subject key back into something a reader recognises.
func describe(subject string) string {
	switch {
	case strings.HasPrefix(subject, "pr:#"):
		return "pull request #" + strings.TrimPrefix(subject, "pr:#")
	case strings.HasPrefix(subject, "pr:"):
		return "pull request " + strings.TrimPrefix(subject, "pr:")
	case strings.HasPrefix(subject, "sha:"):
		return "commit " + strings.TrimPrefix(subject, "sha:")
	}
	return subject
}

// preToolOutput is the deny payload. PreToolUse carries its decision in
// hookSpecificOutput rather than in an exit code, and a call this hook does
// not refuse must emit nothing at all.
type preToolOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

func denyPayload(reason string) string {
	var out preToolOutput
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "deny"
	out.HookSpecificOutput.PermissionDecisionReason = reason
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}
