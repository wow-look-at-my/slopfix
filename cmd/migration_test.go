package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// Each retired marketplace plugin, and what carries its rule now.
type home struct {
	// plugin is the directory that used to hold the rule.
	plugin string
	// command is the subcommand a launcher execs.
	command string
	// rule is the ID passed to --only, or the empty string.
	rule string
}

var migrated = []home{
	{plugin: "ask-properly", command: "ask-properly"},
	{plugin: "enhanced-auto-allow", command: "auto-allow"},
	{plugin: "recommend-go-toolchain", command: "auto-allow"},
	{plugin: "cleanup-bash-cmds", command: "clean-bash"},
	{plugin: "no-busy-poll", command: "busy-poll"},
	{plugin: "no-work-loss", command: "no-work-loss"},
	{plugin: "claude-md-budget", command: "md-budget"},
	{plugin: "link-all-refs", command: "link-refs"},
	{plugin: "no-blame-language", command: "message", rule: blamelanguage.ID},
	{plugin: "detect-permission-seeking", command: "message", rule: laziness.ID},
	{plugin: "no-counts-in-docs", command: "hook", rule: slopfix.IDInventoryCount},
	{plugin: "no-tombstones", command: "hook", rule: tombstones.IDVolume},
	{plugin: "common-checks", command: "workflows"},
}

// Every retired plugin reaches a registered subcommand, and every rule it is
// asked for by name is a rule this binary knows.
func TestEveryRetiredPluginHasAHome(t *testing.T) {
	for _, h := range migrated {
		t.Run(h.plugin, func(t *testing.T) {
			c := find(t, h.command)
			require.NotNil(t, c)
			if h.rule == "" {
				return
			}
			assert.True(t, slopfix.EveryRuleID().Contains(h.rule) || workflow.AllIDs.Contains(h.rule),
				"%s reaches --only %q, which no rule answers to", h.plugin, h.rule)
		})
	}
}

// example-plugin was deleted on purpose. It carried no rule: it was the
// template a new plugin got copied from.
func TestTheExamplePluginIsNotMigrated(t *testing.T) {
	for _, h := range migrated {
		assert.NotEqual(t, "example-plugin", h.plugin)
	}
}
