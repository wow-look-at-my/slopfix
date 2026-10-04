package autoallow

import "strings"

// Refusal denies a command outright, with Message as the reason. Each kind of
// condition it carries must hold, and one value of a kind is enough. A kind
// left empty is not a condition.
type Refusal struct {
	Message string `json:"message"`
	// Flags are matched as whole words. A single-letter short flag also matches inside a group, so `-d` refuses `-fd`.
	Flags []string `json:"flags,omitempty"`
	// FlagValue matches a flag whose value is one of Values, case-insensitively, in the spellings `-X V`, `-XV`.
	FlagValue   *FlagValue `json:"flagValue,omitempty"`
	ArgPrefixes []string   `json:"argPrefixes,omitempty"`
	// ArgSubstrings match inside any argument, so a request body counts.
	ArgSubstrings []string `json:"argSubstrings,omitempty"`
}

type FlagValue struct {
	Flags  []string `json:"flags"`
	Values []string `json:"values"`
}

// matches reads args after the rule's own word.
func (r Refusal) matches(args []string) bool {
	if len(args) > 0 {
		args = args[1:]
	}
	if len(r.Flags) > 0 && !anyArg(args, func(a string) bool { return flagWordMatches(a, r.Flags) }) {
		return false
	}
	if r.FlagValue != nil && !r.FlagValue.matches(args) {
		return false
	}
	if len(r.ArgPrefixes) > 0 && !anyArg(args, func(a string) bool { return hasAnyPrefix(a, r.ArgPrefixes) }) {
		return false
	}
	if len(r.ArgSubstrings) > 0 && !anyArg(args, func(a string) bool { return containsAny(a, r.ArgSubstrings) }) {
		return false
	}
	return len(r.Flags) > 0 || r.FlagValue != nil || len(r.ArgPrefixes) > 0 || len(r.ArgSubstrings) > 0
}

func (fv FlagValue) matches(args []string) bool {
	for i, a := range args {
		for _, f := range fv.Flags {
			var v string
			switch {
			case a == f && i+1 < len(args):
				v = args[i+1]
			case strings.HasPrefix(f, "--") && strings.HasPrefix(a, f+"="):
				v = a[len(f)+1:]
			case !strings.HasPrefix(f, "--") && len(f) == 2 && strings.HasPrefix(a, f) && len(a) > 2:
				v = a[2:]
			default:
				continue
			}
			for _, want := range fv.Values {
				if strings.EqualFold(v, want) {
					return true
				}
			}
		}
	}
	return false
}

func flagWordMatches(arg string, flags []string) bool {
	for _, f := range flags {
		if arg == f || strings.HasPrefix(arg, f+"=") {
			return true
		}
		if len(f) == 2 && f[0] == '-' && f[1] != '-' && isShortGroup(arg) && strings.ContainsRune(arg[1:], rune(f[1])) {
			return true
		}
	}
	return false
}

// isShortGroup is `-abc`: one dash, then letters only.
func isShortGroup(arg string) bool {
	if len(arg) < 3 || arg[0] != '-' || arg[1] == '-' {
		return false
	}
	for _, r := range arg[1:] {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func anyArg(args []string, p func(string) bool) bool {
	for _, a := range args {
		if p(a) {
			return true
		}
	}
	return false
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
