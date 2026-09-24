package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix"
)

// dryRun is the --dry-run flag: report what would go, delete nothing.
var dryRun bool

func init() {
	purgeCmd := &cobra.Command{
		Use:   "purge [dir]",
		Short: "Move CLAUDE.md into AGENTS.md, and delete every other markdown file but README.md",
		Long: "A repository keeps README.md for a person and AGENTS.md for an agent, both at\n" +
			"its root and both under the character budget. Every other .md is deleted.\n\n" +
			"A root CLAUDE.md is renamed to AGENTS.md, or appended to an AGENTS.md that exists.\n" +
			"CLAUDE.md then holds only the line @AGENTS.md, which is how Claude Code reads it.\n\n" +
			"A spec repository, where the prose is the product, opts out with an empty\n" +
			".slopfix-spec file at its root.",
		Args: cobra.MaximumNArgs(1),
		RunE: runPurge,
	}
	purgeCmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted, and delete nothing")
	rootCmd.AddCommand(purgeCmd)
}

func runPurge(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	result, err := slopfix.Purge(root, dryRun)
	if err != nil {
		return err
	}
	switch {
	case result.Agents.Renamed:
		fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s\n", slopfix.ClaudeFile, slopfix.AgentsFile)
	case result.Agents.Merged:
		fmt.Fprintf(cmd.OutOrStdout(), "%s merged into %s\n", slopfix.ClaudeFile, slopfix.AgentsFile)
	}
	for _, path := range result.Deleted {
		fmt.Fprintln(cmd.OutOrStdout(), path)
	}
	if len(result.OverBudget) > 0 {
		fmt.Fprint(cmd.ErrOrStderr(), slopfix.BudgetError(result.OverBudget))
		return errFindings
	}
	// A dry run reports without failing, so a hook can ask what would go.
	return nil
}
