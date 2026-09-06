package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfmt"
)

// dryRun is the --dry-run flag: report what would go, delete nothing.
var dryRun bool

func init() {
	purgeCmd := &cobra.Command{
		Use:   "purge [dir]",
		Short: "Delete every markdown file except README.md and CLAUDE.md at the root",
		Long: "A repository keeps README.md for a person and CLAUDE.md for an agent, both at\n" +
			"its root and both under the character budget. Every other .md is deleted.\n\n" +
			"A spec repository, where the prose is the product, opts out with an empty\n" +
			".slopfmt-spec file at its root.",
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
	result, err := slopfmt.Purge(root, dryRun)
	if err != nil {
		return err
	}
	for _, path := range result.Deleted {
		fmt.Fprintln(cmd.OutOrStdout(), path)
	}
	if len(result.OverBudget) > 0 {
		fmt.Fprint(cmd.ErrOrStderr(), slopfmt.BudgetError(result.OverBudget))
		return errFindings
	}
	// A dry run reports without failing, so a hook can ask what would go.
	return nil
}
