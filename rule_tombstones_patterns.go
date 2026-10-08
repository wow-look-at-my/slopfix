package slopfix

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/ste"
)

// The tombstone wording rules. Each is an entry in rules/english-pattern.xml
// that carries an id. The table names the rule, its phrases and its worked
// examples, and this file registers one rule per entry.
//
// The detection and the repair are the entry's own: the detection reports the
// rule when any of its phrases rewrites the text. The repair applies every
// one of them.
func init() {
	groups := patternGroups()
	ids := set.New[string]()
	for id := range groups {
		ids.Add(id)
	}
	for _, id := range sortedIDs(ids) {
		patterns := groups[id]
		if len(patterns) == 0 {
			continue
		}
		RegisterRule(RuleSpec{
			ID:       id,
			Category: RuleTombstones,
			Detect:   detectPatternEntry(patterns, id),
			Autofix:  autofixPatternEntry(patterns),
			Cases:    patternEntryCases(patterns, id),
		})
	}
}

// detectPatternEntry answers a wording rule's own detection: the rule is
// reported when any phrase of the entry rewrites the case's text.
func detectPatternEntry(patterns []english.Pattern, id string) func(RuleCase) []ste.Finding {
	return detectPattern(patterns, id)
}

// autofixPatternEntry rewrites every phrase the entry carries out of the text.
func autofixPatternEntry(patterns []english.Pattern) func(RuleCase) RuleCase {
	return repairPattern(patterns)
}

// patternEntryCases answers the worked examples the table carries for the
// entry.
func patternEntryCases(patterns []english.Pattern, id string) []RuleCase {
	return patternCases(patterns, id)
}
