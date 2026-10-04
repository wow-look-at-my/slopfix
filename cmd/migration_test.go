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
	// guard is the hook guard that carries the plugin, or the empty string.
	guard string
	// rule is the ID passed to --only, or the empty string.
	rule string
}

var migrated = []home{
	{plugin: "ask-properly", command: "hook", guard: "ask-properly"},
	{plugin: "enhanced-auto-allow", command: "hook", guard: "auto-allow"},
	{plugin: "recommend-go-toolchain", command: "hook", guard: "auto-allow"},
	{plugin: "cleanup-bash-cmds", command: "hook", guard: "clean-bash"},
	{plugin: "no-busy-poll", command: "hook", guard: "busy-poll"},
	{plugin: "no-work-loss", command: "hook", guard: "no-work-loss"},
	{plugin: "claude-md-budget", command: "hook", guard: "md-budget"},
	{plugin: "link-all-refs", command: "hook", guard: "link-refs"},
	{plugin: "no-blame-language", command: "hook", guard: "blame-language", rule: blamelanguage.ID},
	{plugin: "detect-permission-seeking", command: "hook", guard: "laziness", rule: laziness.ID},
	{plugin: "no-counts-in-docs", command: "hook", rule: slopfix.IDInventoryCount},
	{plugin: "no-tombstones", command: "hook", rule: tombstones.IDVolume},
	{plugin: "common-checks", command: "check"},
}

// Every retired plugin reaches a registered subcommand and a guard that hook
// runs, and every rule it is asked for by name is a rule this binary knows.
func TestEveryRetiredPluginHasAHome(t *testing.T) {
	for _, h := range migrated {
		t.Run(h.plugin, func(t *testing.T) {
			c := find(t, h.command)
			require.NotNil(t, c)
			if h.guard != "" {
				assert.Contains(t, guardNames(), h.guard)
			}
			if h.rule == "" {
				return
			}
			assert.True(t, slopfix.EveryRuleID().Contains(h.rule) || workflow.AllIDs.Contains(h.rule),
				"%s reaches --only %q, which no rule answers to", h.plugin, h.rule)
		})
	}
}

// example-plugin was deleted on purpose. It carried no rule.
func TestTheExamplePluginIsNotMigrated(t *testing.T) {
	for _, h := range migrated {
		assert.NotEqual(t, "example-plugin", h.plugin)
	}
}
