package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/askproperly"
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/ste"
)

// messageIDs names every rule that judges a closing message, so a typo is
// rejected rather than reading as a clean message.
func messageIDs() set.Set[string] {
	return set.Of(laziness.ID, blamelanguage.ID, askproperly.ID)
}

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

// messageFindings judges a closing message with every rule the selection runs.
func messageFindings(message string, runs func(string) bool) []ste.Finding {
	var out []ste.Finding
	if runs(laziness.ID) {
		for _, hit := range laziness.Check(message) {
			out = append(out, ste.Finding{Line: hit.Line, ID: hit.ID, Rule: hit.Tell, Detail: hit.Sentence,
				Fix: "Do the work the sentence hands back, then say what you did."})
		}
	}
	if runs(blamelanguage.ID) {
		for _, hit := range blamelanguage.Check(message) {
			detail := hit.Phrase
			if detail == "" {
				detail = hit.Sentence
			}
			out = append(out, ste.Finding{Line: hit.Line, ID: hit.ID, Rule: hit.Tell, Detail: detail,
				Fix: "Own the defect and say what you fixed."})
		}
	}
	if runs(askproperly.ID) {
		for _, hit := range askproperly.FindQuestions(message) {
			out = append(out, ste.Finding{Line: lineOf(message, hit.Line), ID: askproperly.ID,
				Rule: "a decision handed to the reader in prose", Detail: strings.TrimSpace(hit.Text),
				Fix: "Make the decision and say what you assumed."})
		}
	}
	return out
}

// lineOf answers the line number, counting from one, of the first line in text
// that equals line. A line it cannot find answers one.
func lineOf(text, line string) int {
	for i, candidate := range strings.Split(text, "\n") {
		if strings.TrimSpace(candidate) == strings.TrimSpace(line) {
			return i + 1
		}
	}
	return 1
}
