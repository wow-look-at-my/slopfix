// flags.go holds the flag tests a rule's allowed, required and denied lists use.

package autoallow

import "strings"

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

// hasAnyFlagWord is hasAnyFlag that also finds a single-letter flag inside a cluster: `sort -uo out` carries -o.
func hasAnyFlagWord(args []string, flags []string) bool {
	for _, a := range args {
		if flagWordMatches(a, flags) {
			return true
		}
	}
	return false
}
