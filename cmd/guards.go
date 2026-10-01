package cmd

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/askproperly"
	"github.com/wow-look-at-my/slopfix/autoallow"
	"github.com/wow-look-at-my/slopfix/bashclean"
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/busypoll"
	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/linkrefs"
	"github.com/wow-look-at-my/slopfix/mdbudget"
	"github.com/wow-look-at-my/slopfix/noworkloss"
)

// The events a guard serves, as Claude Code spells them on the payload.
const (
	eventPreToolUse        = "PreToolUse"
	eventPermissionRequest = "PermissionRequest"
	eventPostToolUse       = "PostToolUse"
	eventSessionStart      = "SessionStart"
	eventStop              = "Stop"
	eventMessageDisplay    = "MessageDisplay"
)

// hookResult is what a guard answers: the stdout, the stderr and the exit code
// that Claude Code reads.
type hookResult struct {
	Stdout string
	Stderr string
	Code   int
}

// guard is a single check that `hook` runs on the events it serves.
type guard struct {
	name   string
	events []string
	run    func(payload []byte, write writeSelection) hookResult
}

// writeSelection is what the write guard reads from the flags.
type writeSelection struct {
	rules []slopfix.Rule
	ids   []string
}

// writeGuard is the name of the guard that repairs the text a write adds.
const writeGuard = "write"

// guards is every check `hook` runs, in the order it runs them on an event.
// The Bash rewrite runs first, so each later guard judges the command that
// will run.
var guards = []guard{
	{name: "clean-bash", events: []string{eventPreToolUse}, run: reader(func(r *strings.Reader) hookResult { return hookResult(bashclean.Run(r)) })},
	{name: writeGuard, events: []string{eventPreToolUse}, run: func(payload []byte, write writeSelection) hookResult {
		return hookResult{Stdout: judge(payload, write.rules, write.ids)}
	}},
	{name: "no-work-loss", events: []string{eventPreToolUse}, run: reader(func(r *strings.Reader) hookResult { return hookResult(noworkloss.Run(r)) })},
	{name: "auto-allow", events: []string{eventPreToolUse, eventPermissionRequest}, run: reader(func(r *strings.Reader) hookResult { return hookResult(autoallow.Run(r)) })},
	{name: "busy-poll", events: []string{eventPreToolUse, eventStop}, run: reader(func(r *strings.Reader) hookResult { return hookResult(busypoll.Run(r)) })},
	{name: "md-budget", events: []string{eventSessionStart, eventPostToolUse, eventStop}, run: reader(func(r *strings.Reader) hookResult { return hookResult(mdbudget.Run(r)) })},
	{name: "laziness", events: []string{eventStop}, run: reader(func(r *strings.Reader) hookResult { return hookResult(laziness.Run(r)) })},
	{name: "link-refs", events: []string{eventMessageDisplay}, run: reader(func(r *strings.Reader) hookResult { return hookResult(linkrefs.Run(r)) })},
	{name: "blame-language", events: []string{eventMessageDisplay}, run: reader(func(r *strings.Reader) hookResult { return hookResult(blamelanguage.Run(r)) })},
	{name: "ask-properly", events: []string{eventMessageDisplay}, run: reader(func(r *strings.Reader) hookResult { return hookResult(askproperly.Run(r)) })},
}

// reader adapts a package entry point that reads its payload from a stream.
func reader(run func(*strings.Reader) hookResult) func([]byte, writeSelection) hookResult {
	return func(payload []byte, _ writeSelection) hookResult {
		return run(strings.NewReader(string(payload)))
	}
}

// guardNames answers every guard a caller may name to --only.
func guardNames() []string {
	names := make([]string, 0, len(guards))
	for _, g := range guards {
		names = append(names, g.name)
	}
	return names
}

// hookSelection turns --only into the guards to run and the rules the write
// guard applies. An empty list runs every guard with every rule.
//
// An entry is a guard name, a rule category or a rule ID. A rule turns the
// write guard on. An unknown name is an error, because a guard that runs
// nothing reads as a clean call.
func hookSelection(only []string) (set.Set[string], writeSelection, error) {
	if len(only) == 0 {
		return set.Of(guardNames()...), writeSelection{}, nil
	}
	running := set.New[string]()
	var ruleNames []string
	for _, name := range only {
		name = strings.TrimSpace(name)
		if slices.Contains(guardNames(), name) {
			running.Add(name)
			continue
		}
		ruleNames = append(ruleNames, name)
	}
	var write writeSelection
	if len(ruleNames) > 0 {
		rules, ids, err := selectedRules(ruleNames)
		if err != nil {
			return nil, writeSelection{}, fmt.Errorf("%w. A guard is one of %s", err, strings.Join(guardNames(), ", "))
		}
		write = writeSelection{rules: rules, ids: ids}
		running.Add(writeGuard)
	}
	return running, write, nil
}

// eventOf reads the event a payload is for. A payload with a tool and no event
// name is a PreToolUse payload that a person built by hand.
func eventOf(payload []byte) string {
	var in struct {
		HookEventName string `json:"hook_event_name"`
		ToolName      string `json:"tool_name"`
	}
	if json.Unmarshal(payload, &in) != nil {
		return ""
	}
	if in.HookEventName == "" && in.ToolName != "" {
		return eventPreToolUse
	}
	return in.HookEventName
}

// dispatch runs every selected guard that serves the payload's event and
// merges what they answer into one response.
func dispatch(payload []byte, running set.Set[string], write writeSelection) hookResult {
	event := eventOf(payload)
	var serving []guard
	for _, g := range guards {
		if running.Contains(g.name) && slices.Contains(g.events, event) {
			serving = append(serving, g)
		}
	}
	switch event {
	case eventPreToolUse:
		return preToolUse(payload, serving, write)
	case eventMessageDisplay:
		return messageDisplay(payload, serving, write)
	}
	results := make([]hookResult, 0, len(serving))
	for _, g := range serving {
		results = append(results, g.run(payload, write))
	}
	return merge(event, results)
}

// preToolUse runs the guards one after another. The first refusal is the
// answer. A rewrite of the tool input reaches every guard after it, so no
// guard judges a command that will not run.
func preToolUse(payload []byte, serving []guard, write writeSelection) hookResult {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(payload, &envelope) != nil {
		return hookResult{}
	}
	var results []hookResult
	var updated json.RawMessage
	for _, g := range serving {
		res := g.run(payload, write)
		if res.Code != 0 {
			return res
		}
		spec, _ := decode(res.Stdout)["hookSpecificOutput"].(map[string]any)
		if spec["permissionDecision"] == "deny" {
			return res
		}
		if input, ok := spec["updatedInput"]; ok {
			if raw, err := json.Marshal(input); err == nil {
				envelope["tool_input"] = raw
				if next, err := json.Marshal(envelope); err == nil {
					payload, updated = next, raw
				}
			}
		}
		results = append(results, res)
	}
	merged := merge(eventPreToolUse, results)
	if updated == nil || merged.Code != 0 {
		return merged
	}
	// The last rewrite already holds every rewrite before it.
	out := decode(merged.Stdout)
	if out == nil {
		out = map[string]any{}
	}
	spec, _ := out["hookSpecificOutput"].(map[string]any)
	if spec == nil {
		spec = map[string]any{"hookEventName": eventPreToolUse}
	}
	spec["updatedInput"] = updated
	out["hookSpecificOutput"] = spec
	merged.Stdout = encode(out)
	return merged
}

// messageDisplay composes what the reader sees. A guard that rewrites the
// text replaces the base. A guard that appends a note adds its note.
func messageDisplay(payload []byte, serving []guard, write writeSelection) hookResult {
	var in struct {
		Delta string `json:"delta"`
	}
	if json.Unmarshal(payload, &in) != nil {
		return hookResult{}
	}
	base := in.Delta
	var notes, stderr []string
	for _, g := range serving {
		res := g.run(payload, write)
		if res.Stderr != "" {
			stderr = append(stderr, res.Stderr)
		}
		spec, _ := decode(res.Stdout)["hookSpecificOutput"].(map[string]any)
		content, _ := spec["displayContent"].(string)
		if content == "" {
			continue
		}
		if note, ok := strings.CutPrefix(content, in.Delta); ok {
			notes = append(notes, note)
			continue
		}
		base = content
	}
	result := hookResult{Stderr: strings.Join(stderr, "")}
	if base == in.Delta && len(notes) == 0 {
		return result
	}
	result.Stdout = encode(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":  eventMessageDisplay,
		"displayContent": base + strings.Join(notes, ""),
	}})
	return result
}

// merge joins what several guards answered to one event. A single answer
// passes through as it is. Several refusals become one refusal that carries
// every reason. Otherwise the context and the messages are joined.
func merge(event string, results []hookResult) hookResult {
	var spoke []hookResult
	for _, res := range results {
		if res.Stdout != "" || res.Stderr != "" || res.Code != 0 {
			spoke = append(spoke, res)
		}
	}
	switch len(spoke) {
	case 0:
		return hookResult{}
	case 1:
		return spoke[0]
	}

	var reasons, stderr, contexts, messages []string
	out := map[string]any{}
	spec := map[string]any{}
	for _, res := range spoke {
		if res.Code == 2 {
			reasons = append(reasons, strings.TrimSpace(res.Stderr))
		} else if res.Stderr != "" {
			stderr = append(stderr, res.Stderr)
		}
		decoded := decode(res.Stdout)
		if decoded["decision"] == "block" {
			reason, _ := decoded["reason"].(string)
			reasons = append(reasons, strings.TrimSpace(reason))
			continue
		}
		for key, value := range decoded {
			switch key {
			case "hookSpecificOutput":
				inner, _ := value.(map[string]any)
				for k, v := range inner {
					if text, ok := v.(string); ok && k == "additionalContext" {
						contexts = append(contexts, text)
						continue
					}
					spec[k] = v
				}
			case "systemMessage":
				if text, ok := value.(string); ok {
					messages = append(messages, text)
				}
			default:
				out[key] = value
			}
		}
	}
	if len(reasons) > 0 {
		return hookResult{Stderr: strings.Join(reasons, "\n\n") + "\n", Code: 2}
	}
	if len(contexts) > 0 {
		spec["additionalContext"] = strings.Join(contexts, "\n\n")
	}
	if len(spec) > 0 {
		spec["hookEventName"] = event
		out["hookSpecificOutput"] = spec
	}
	if len(messages) > 0 {
		out["systemMessage"] = strings.Join(messages, "\n")
	}
	result := hookResult{Stderr: strings.Join(stderr, "")}
	if len(out) > 0 {
		result.Stdout = encode(out)
	}
	return result
}

// decode reads a guard's stdout as a JSON object. Anything else reads as nil.
func decode(stdout string) map[string]any {
	var out map[string]any
	if json.Unmarshal([]byte(stdout), &out) != nil {
		return nil
	}
	return out
}

func encode(v map[string]any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}
