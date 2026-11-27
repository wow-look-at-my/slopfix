// library.go is the shipped table as a library, for a program that keeps its own permission chain.

package autoallow

import (
	"strings"
	"sync"
)

var shipped = sync.OnceValues(func() (Rules, error) { return loadXMLRules(rulesXML) })

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

// quoteArgv writes argv as one shell statement whose words parse back to argv.
func quoteArgv(argv []string) string {
	words := make([]string, len(argv))
	for i, a := range argv {
		words[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(words, " ")
}
