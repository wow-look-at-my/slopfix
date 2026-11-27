// library.go is the shipped table as a library, for a program that keeps its own permission chain.

package autoallow

import (
	"slices"
	"strings"
	"sync"
)

var shipped = sync.OnceValues(func() (Rules, error) { return loadXMLRules(rulesXML) })

// UnknownWord stands for an argument the caller cannot read statically, such as "$BRANCH".
const UnknownWord = "\x00unknown"

// Allowed reports whether the shipped table allows one simple command. argv[0] is the program word and the rest are
// its arguments, each after the shell expanded it. A deny or ask rule that matches makes the answer false, and so
// does a process rule that names the program. A wholeCommand rule judges a whole command line, so it plays no part.
func Allowed(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	t, err := shipped()
	if err != nil {
		return false
	}
	var procs []CommandRule
	for _, r := range append(append([]CommandRule{}, t.DenyCommands...), t.AskCommands...) {
		if !r.WholeOnly {
			procs = append(procs, r)
		}
	}
	if name, _ := matchCommandRule(quoteArgv(argv), procs); name != "" {
		return false
	}
	// An unknown word could carry whatever a deny or ask rule of the program looks for.
	if slices.Contains(argv[1:], UnknownWord) && (namesAny(t.Deny, argv[0]) || namesAny(t.Ask, argv[0])) {
		return false
	}
	for _, nodes := range [][]CommandNode{t.Deny, t.Ask} {
		if d, _ := evaluateArgs(argv, nodes, t.Allow); d == "allow" || d == "deny" {
			return false
		}
	}
	d, _ := evaluateArgs(argv, t.Allow, t.Allow)
	return d == "allow"
}

// AllowedMCP reports whether the shipped table trusts this tool of this MCP server.
func AllowedMCP(server, tool string) bool {
	t, err := shipped()
	return err == nil && matchMCPServer(t.MCPServers, server, tool)
}

// quoteArgv writes argv as one shell statement whose words parse back to argv. An unknown word becomes a plain
// operand, because a process rule judges the program and not its operands.
func quoteArgv(argv []string) string {
	words := make([]string, len(argv))
	for i, a := range argv {
		if a == UnknownWord {
			a = "unknown"
		}
		words[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(words, " ")
}

// namesAny reports whether a rule of nodes names the program.
func namesAny(nodes []CommandNode, program string) bool {
	for _, n := range nodes {
		if matchesName(n.Name, program) {
			return true
		}
	}
	return false
}

// admitsEveryArgument reports whether a rule asserts that any argument is harmless: every flag allowed, and nothing
// that narrows the rule by what its arguments say.
func admitsEveryArgument(n CommandNode) bool {
	return n.AllowedFlags == "*" && len(n.DeniedFlags) == 0 && len(n.RequiredFlags) == 0 &&
		len(n.DenyArgSubstrings) == 0 && len(n.AllowedArgPrefixes) == 0 && len(n.ExecFlags) == 0 &&
		len(n.Refusals) == 0 && n.RequireFlagValue == nil && n.MaxPositionals == nil && !n.BareOnly
}
