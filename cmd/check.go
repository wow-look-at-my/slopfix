package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix"
)

// writeRepaired puts a repair back, keeping the mode the file already had.
func writeRepaired(path, text string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), info.Mode().Perm())
}

// checkFix asks check to repair what it can rather than only report it.
var checkFix bool

func init() {
	check := &cobra.Command{
		Use:   "check [--fix] <file>...",
		Short: "Report what every rule rejects, and with --fix repair what it can",
		// The names the rules answered to one at a time. A file decides which
		// rules read it, so asking by name selected nothing this does not.
		Aliases: []string{"comments", "workflows", "fix"},
		Args:    cobra.MinimumNArgs(1),
		RunE:    runCheck,
	}
	check.Flags().BoolVar(&checkFix, "fix", false, "write the repair back to each file")
	rootCmd.AddCommand(check)
}

func runCheck(cmd *cobra.Command, args []string) error {
	// Invoked as "fix", the repair is what was asked for, flag or no flag.
	repairing := checkFix || cmd.CalledAs() == "fix"
	found := false
	for _, path := range args {
		if repairing {
			repair, err := slopfix.FixFile(path)
			if err != nil {
				return err
			}
			if repair.Changed {
				if err := writeRepaired(path, repair.Text); err != nil {
					return err
				}
			}
			for _, finding := range repair.Findings {
				found = true
				fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", path, finding)
			}
			continue
		}
		findings, err := slopfix.CheckFile(path)
		if err != nil {
			return err
		}
		for _, finding := range findings {
			found = true
			fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", path, finding)
		}
	}
	if found {
		return errFindings
	}
	return nil
}
