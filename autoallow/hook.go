// Every tool fills the keys its own shape has and leaves the rest empty, so this reads the location out of whichever tool is asking.

package autoallow

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// Hook input from Claude Code
type HookInput struct {
	HookEventName string    `json:"hook_event_name"`
	ToolName      string    `json:"tool_name"`
	Cwd           string    `json:"cwd"`
	ToolInput     ToolInput `json:"tool_input"`
}

// Every tool fills the keys its own shape has and leaves the rest empty, so
// this reads the location out of whichever tool is asking.
type ToolInput struct {
	Command      string `json:"command"`
	FilePath     string `json:"file_path"`
	NotebookPath string `json:"notebook_path"`
	Path         string `json:"path"`
	Pattern      string `json:"pattern"`
}

// Sections evaluated deny > ask > allow. A rule matches argv as written
// (CommandNode) or the resolved process name (CommandRule).
type Rules struct {
	Allow []CommandNode `json:"allow"`
	Ask   []CommandNode `json:"ask"`
	Deny  []CommandNode `json:"deny"`

	AllowCommands []CommandRule `json:"allowCommands"`
	AskCommands   []CommandRule `json:"askCommands"`
	DenyCommands  []CommandRule `json:"denyCommands"`

	MCPServers map[string][]string `json:"mcpServers"`

	// DenyPaths refuses by location, whichever tool is asking.
	DenyPaths []PathRule `json:"denyPaths"`
}

type CommandNode struct {
	Name               interface{}      `json:"name"` // string or []string
	Description        string           `json:"description,omitempty"`
	AllowedFlags       interface{}      `json:"allowedFlags,omitempty"` // "*" or []string
	DeniedFlags        []string         `json:"deniedFlags,omitempty"`
	ExecFlags          []string         `json:"execFlags,omitempty"`
	RequiredFlags      []string         `json:"requiredFlags,omitempty"`
	RequireFlagValue   *RequireFlagRule `json:"requireFlagValue,omitempty"`
	DenyWithMessage    string           `json:"denyWithMessage,omitempty"`
	FlagsWithValue     []string         `json:"flagsWithValue,omitempty"`
	HelpAlwaysAllowed  bool             `json:"helpAlwaysAllowed,omitempty"`
	BareOnly          bool             `json:"bareOnly,omitempty"`
	// MaxPositionals caps the operands.
	MaxPositionals     *int          `json:"maxPositionals,omitempty"`
	DenyArgSubstrings  []string         `json:"denyArgSubstrings,omitempty"`
	Refusals           []Refusal        `json:"refusals,omitempty"`
	AllowedArgPrefixes []string         `json:"allowedArgPrefixes,omitempty"`
	Subcommands        []CommandNode    `json:"subcommands,omitempty"`
}

type RequireFlagRule struct {
	Flags   []string `json:"flags"`
	Default string   `json:"default"`
	Allowed []string `json:"allowed"`
}

// The events this binary answers. They are NOT interchangeable; the deny half
// rides PreToolUse. See the auto-allow section of AGENTS.md
const (
	eventPermissionRequest = "PermissionRequest"
	eventPreToolUse        = "PreToolUse"
)

// Permission response (PermissionRequest event).
type PermissionResponse struct {
	HookSpecificOutput struct {
		HookEventName string `json:"hookEventName"`
		Decision      struct {
			Behavior string `json:"behavior"`
			Message  string `json:"message,omitempty"`
		} `json:"decision"`
	} `json:"hookSpecificOutput"`
}

// PreToolUse response: a flat permissionDecision, not the nested object. The
// CLI rejects a hookEventName that is not the event it dispatched, so the shape
// follows the event rather than the verdict.
type PreToolUseResponse struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

var rules Rules

func evaluateCommand(command string) (string, string) {
	return evaluateCommandWith(command, rules)
}

// evaluateCommandWith takes the rule set as a value, so a test never swaps the package-level `rules`.
func evaluateCommandWith(command string, rules Rules) (string, string) {
	// Process rules outrank command rules, and are answered by walking the parse
	// tree. They still see a command the allow path below refuses to read -- a
	// `$(...)`, a subshell, anything with a redirect.
	for _, section := range []struct {
		rules    []CommandRule
		behavior string
	}{
		{rules.DenyCommands, "deny"},
		{rules.AskCommands, "ask"},
	} {
		if name, msg := matchCommandRule(command, section.rules); name != "" {
			if msg == "" {
				msg = name + " may not be run here."
			}
			return section.behavior, msg
		}
	}

	commands := parseAllCommands(command)
	if len(commands) == 0 {
		return "", ""
	}

	// Every command in a compound must clear the bar: any denied denies, all
	// allowed allows, anything else passes through to the permission flow.
	for _, section := range []struct {
		nodes    []CommandNode
		behavior string
	}{
		{rules.Deny, "deny"},
		{rules.Ask, "ask"},
	} {
		for _, args := range commands {
			if decision, msg := evaluateArgs(args, section.nodes, rules.Allow); decision == "allow" || decision == "deny" {
				// The section supplies the verdict, not the rule.
				return section.behavior, msg
			}
		}
	}

	allAllowed := true
	for _, args := range commands {
		decision, msg := evaluateArgs(args, rules.Allow, rules.Allow)
		if decision == "deny" {
			return "deny", msg
		}
		if decision != "allow" {
			allAllowed = false
		}
	}

	if allAllowed {
		if name, _ := matchCommandRule(command, rules.AllowCommands); name != "" {
			return "allow", ""
		}
		return "allow", ""
	}
	return "", ""
}

// `allow` is the top-level allow set, carried down for the exec-flag
// recursion: a command run through `find -exec` clears the same bar.
func evaluateArgs(args []string, nodes, allow []CommandNode) (string, string) {
	if len(args) == 0 || len(nodes) == 0 {
		return "", ""
	}

	current := args[0]
	remaining := args[1:]

	// Merge every matching node: deny beats allow beats passthrough.
	anyAllowed := false
	for _, node := range nodes {
		if !matchesName(node.Name, current) {
			continue
		}

		decision, msg := evaluateOneNode(node, args, remaining, allow)
		if decision == "deny" {
			return "deny", msg
		}
		if decision == "allow" {
			anyAllowed = true
		}
	}

	if anyAllowed {
		return "allow", ""
	}
	return "", ""
}

func evaluateOneNode(node CommandNode, args []string, remaining []string, allow []CommandNode) (string, string) {
	// If helpAlwaysAllowed, any subcommand chain ending in --help/-h is allowed
	if node.HelpAlwaysAllowed && hasAnyFlag(remaining, []string{"--help", "-h"}) {
		return "allow", ""
	}

	if node.DenyWithMessage != "" {
		return "deny", node.DenyWithMessage
	}

	// A refusal is a verdict of its own: the command is ordinary, and what its
	// arguments carry decides it.
	for _, r := range node.Refusals {
		if r.matches(args) {
			return "deny", r.Message
		}
	}

	// A denied substring unmatches the node: in a script argument (awk, sed)
	// a dangerous feature appears inside the body, not as its own word.
	if len(node.DenyArgSubstrings) > 0 {
		for _, arg := range args {
			for _, substr := range node.DenyArgSubstrings {
				if strings.Contains(arg, substr) {
					return "", ""
				}
			}
		}
	}

	// A required flag admits the command to the checks below. It does not skip them: `tar -t` reads, and
	// `tar -t -x` still extracts.
	if len(node.RequiredFlags) > 0 && !hasAnyFlagWord(args, node.RequiredFlags) {
		return "", ""
	}

	// If bareOnly, only allow when there are no remaining arguments
	if node.BareOnly {
		if len(remaining) == 0 {
			return "allow", ""
		}
		return "", ""
	}

	// Check requireFlagValue
	if node.RequireFlagValue != nil {
		values := getFlagValues(args, node.RequireFlagValue.Flags)
		if len(values) == 0 {
			values = []string{node.RequireFlagValue.Default}
		}
		for _, value := range values {
			if !slices.Contains(node.RequireFlagValue.Allowed, value) {
				return "", ""
			}
		}
		// A rule that also lists its allowed flags is held to them below.
		if node.AllowedFlags == nil {
			return "allow", ""
		}
	}

	// Strip own flags (that take values) before subcommand matching
	subcommandArgs := remaining
	if len(node.FlagsWithValue) > 0 {
		subcommandArgs = stripFlagsWithValue(remaining, node.FlagsWithValue)
	}

	// A rule with subcommands skips the leading flags it allows, then the next
	// word must be one of its subcommands. `git -C dir log` and `git --no-pager
	// diff` reach their subcommand.
	if len(node.Subcommands) > 0 {
		lead := skipAllowedFlags(subcommandArgs, node.AllowedFlags)
		if len(lead) > 0 && !strings.HasPrefix(lead[0], "-") {
			return evaluateArgs(lead, node.Subcommands, allow)
		}
	}

	if len(node.DeniedFlags) > 0 && hasAnyFlagWord(args, node.DeniedFlags) {
		return "", ""
	}

	if node.MaxPositionals != nil {
		_, positionals := splitFlagsAndArgs(subcommandArgs)
		if len(positionals) > *node.MaxPositionals {
			return "", ""
		}
	}

	// Check exec flags: extract sub-commands and evaluate them
	if len(node.ExecFlags) > 0 {
		subCmds := extractExecSubCommands(remaining, node.ExecFlags)
		for _, subCmd := range subCmds {
			decision, msg := evaluateArgs(subCmd, allow, allow)
			if decision == "deny" {
				return "deny", msg
			}
			if decision != "allow" {
				return "", ""
			}
		}
	}

	// A rule that names required flags and no allowed flags allows once the checks above pass.
	if node.AllowedFlags == nil && len(node.RequiredFlags) > 0 {
		return "allow", ""
	}

	// Check allowed flags (and optionally constrain positional args by prefix)
	if node.AllowedFlags != nil {
		allowedFlags := withRequired(node.AllowedFlags, node.RequiredFlags)
		effectiveRemaining := remaining
		if len(node.FlagsWithValue) > 0 {
			effectiveRemaining = stripFlagsWithValue(remaining, node.FlagsWithValue)
		}

		if len(node.AllowedArgPrefixes) > 0 {
			flags, positionals := splitFlagsAndArgs(effectiveRemaining)
			if len(positionals) == 0 {
				return "", ""
			}
			if !allArgsMatchPrefix(positionals, node.AllowedArgPrefixes) {
				return "", ""
			}
			if checkAllowedFlags(flags, allowedFlags) {
				return "allow", ""
			}
		} else if checkAllowedFlags(remaining, allowedFlags) {
			return "allow", ""
		}
	}

	return "", ""
}

// withRequired adds the required flags to an allowed-flags list. A required flag is one the rule expects to see.
// "*" stays "*".
func withRequired(allowed interface{}, required []string) interface{} {
	list, ok := allowed.([]interface{})
	if !ok || len(required) == 0 {
		return allowed
	}
	out := append([]interface{}{}, list...)
	for _, f := range required {
		out = append(out, f)
	}
	return out
}

// skipAllowedFlags drops the leading flags an allowed-flags list admits: every flag for "*", the listed ones for a
// list. What is left starts at the first word the list does not admit.
func skipAllowedFlags(args []string, allowed interface{}) []string {
	list, isList := allowed.([]interface{})
	any := allowed == "*"
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		ok := any
		for _, f := range list {
			if s, _ := f.(string); s == args[0] || strings.HasPrefix(args[0], s+"=") {
				ok = true
				break
			}
		}
		if !ok || (!any && !isList) {
			return args
		}
		args = args[1:]
	}
	return args
}

// hasAnyFlagWord is hasAnyFlag that also finds a single-letter flag inside a
// cluster: `sort -uo out` carries -o.
func hasAnyFlagWord(args []string, flags []string) bool {
	for _, a := range args {
		if flagWordMatches(a, flags) {
			return true
		}
	}
	return false
}

func matchesName(nameField interface{}, target string) bool {
	switch v := nameField.(type) {
	case string:
		return v == target
	case []interface{}:
		for _, name := range v {
			if s, ok := name.(string); ok && s == target {
				return true
			}
		}
	}
	return false
}

func checkAllowedFlags(args []string, allowedFlags interface{}) bool {
	switch v := allowedFlags.(type) {
	case string:
		return v == "*"
	case []interface{}:
		allowed := set.New[string]()
		for _, f := range v {
			if s, ok := f.(string); ok {
				allowed.Add(s)
			}
		}
		// Check all flags are allowed
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") && !allowed.Contains(arg) {
				return false
			}
		}
		return true
	}
	return false
}

func hasAnyFlag(args []string, flags []string) bool {
	flagSet := set.New[string]()
	for _, f := range flags {
		flagSet.Add(f)
	}
	for _, arg := range args {
		if flagSet.Contains(arg) {
			return true
		}
		// Also match --flag=value form.
		if idx := strings.Index(arg, "="); idx > 0 && flagSet.Contains(arg[:idx]) {
			return true
		}
	}
	return false
}

func stripFlagsWithValue(args []string, flags []string) []string {
	flagSet := set.New[string]()
	for _, f := range flags {
		flagSet.Add(f)
	}
	var result []string
	for i := 0; i < len(args); i++ {
		if flagSet.Contains(args[i]) && i+1 < len(args) {
			i++ // skip the value too
			continue
		}
		result = append(result, args[i])
	}
	return result
}

// getFlagValues returns the value of every occurrence of any of flags, in the
// spellings `-X V`, `--long=V` and `-XV`. A flag given as the last word, with
// no value after it, yields an empty value.
func getFlagValues(args []string, flags []string) []string {
	var values []string
	for i, arg := range args {
		for _, f := range flags {
			switch {
			case arg == f && i+1 < len(args):
				values = append(values, args[i+1])
			case arg == f:
				values = append(values, "")
			case strings.HasPrefix(f, "--") && strings.HasPrefix(arg, f+"="):
				values = append(values, arg[len(f)+1:])
			case len(f) == 2 && f[0] == '-' && f[1] != '-' && strings.HasPrefix(arg, f) && len(arg) > 2:
				values = append(values, arg[2:])
			}
		}
	}
	return values
}

func splitFlagsAndArgs(args []string) (flags, positionals []string) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
		} else {
			positionals = append(positionals, arg)
		}
	}
	return
}

func allArgsMatchPrefix(args []string, prefixes []string) bool {
	for _, arg := range args {
		matched := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(arg, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// decisionPayload renders the response for the event it is answering. The CLI
// rejects a payload whose hookEventName is not the event it dispatched, so the
// shape follows the event rather than the verdict.
func decisionPayload(event, behavior, message string) string {
	var v any
	if event == eventPreToolUse {
		resp := PreToolUseResponse{}
		resp.HookSpecificOutput.HookEventName = eventPreToolUse
		resp.HookSpecificOutput.PermissionDecision = behavior
		resp.HookSpecificOutput.PermissionDecisionReason = message
		v = resp
	} else {
		resp := PermissionResponse{}
		resp.HookSpecificOutput.HookEventName = eventPermissionRequest
		resp.HookSpecificOutput.Decision.Behavior = behavior
		if message != "" {
			resp.HookSpecificOutput.Decision.Message = message
		}
		v = resp
	}
	out, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(out) + "\n"
}
