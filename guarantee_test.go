package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// everyRule selects every message rule.
func everyRule(string) bool { return true }

// messageErrors answers every error check reports in a closing message: the
// message rules, and every rule that reads text with no path.
func messageErrors(text string) []string {
	var out []string
	for _, f := range slopfix.CheckMessage(text, everyRule) {
		if !f.Warning() {
			out = append(out, "message: "+f.String())
		}
	}
	for _, f := range slopfix.CheckContent("", text) {
		if !f.Warning() {
			out = append(out, "message: "+f.String())
		}
	}
	rep := slopfix.Report(slopfix.Request{Content: text, MaxCommentLines: tombstones.DefaultMaxCommentLines})
	for _, f := range rep.Findings {
		if !f.Warning() {
			out = append(out, "message: "+f.String())
		}
	}
	for _, k := range rep.Kept {
		out = append(out, "message: "+k.ID+" "+k.Phrase)
	}
	return out
}

// fixMessage repairs a closing message with every rule, until a pass changes nothing.
func fixMessage(text string) string {
	for range 8 {
		next := slopfix.FixMessage(text, everyRule)
		next = slopfix.Fix(slopfix.Request{Content: next, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
		if next == text {
			return text
		}
		text = next
	}
	return text
}

// The guarantee, read from the registry and nothing else. For every case of
// every rule, `slopfix fix` with every rule leaves nothing `slopfix check`
// reports as an error. A rule registers only with a case, so no rule escapes.
func TestFixLeavesNoErrorInAnyCaseOfAnyRule(t *testing.T) {
	for _, rule := range slopfix.AllRuleSpecs() {
		t.Run(rule.ID, func(t *testing.T) {
			for _, raw := range rule.Cases {
				fixture, err := slopfix.Materialize(t.TempDir(), raw)
				require.NoError(t, err)
				req := slopfix.Request{MaxCommentLines: tombstones.DefaultMaxCommentLines}

				fixed := slopfix.FixTreeWith(fixture.Root, req)
				assert.Empty(t, fixed.Unmet, "%s: case %q", rule.ID, raw.Name)
				left := errorLines(slopfix.CheckTreeWith(fixture.Root, req))
				if fixture.Text != "" {
					left = append(left, messageErrors(fixMessage(fixture.Text))...)
				}
				assert.Empty(t, left, "%s: case %q: fix left these errors", rule.ID, raw.Name)
			}
		})
	}
}
