package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
)

// messageIDs names every rule that judges a closing message.
func messageIDs() set.Set[string] { return slopfix.MessageIDs() }

// family is the part of a rule ID before the slash, which a caller writes to
// select everything under that heading.
func family(id string) string {
	before, _, _ := strings.Cut(id, "/")
	return before
}

// messageSelection checks --only against the message rules, and answers
// whether a rule runs. An empty list runs every rule.
func messageSelection(only []string) (func(string) bool, error) {
	known := messageIDs()
	for _, want := range only {
		if known.Contains(want) {
			continue
		}
		if slices.ContainsFunc(slices.Collect(known.All()), func(id string) bool { return family(id) == want }) {
			continue
		}
		return nil, fmt.Errorf("unknown message rule %q: choose from %s, or a family before the slash", want, slopfix.Listed(known))
	}
	return func(id string) bool {
		return len(only) == 0 || slices.Contains(only, id) || slices.Contains(only, family(id))
	}, nil
}
