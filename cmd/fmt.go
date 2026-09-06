package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfmt"
)

// listOnly is the -l flag: name the files a rewrite would change, change none.
var listOnly bool

func init() {
	fmtCmd := &cobra.Command{
		Use:   "fmt <file>...",
		Short: "Rewrite each file so every paragraph is one line",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runFmt,
	}
	fmtCmd.Flags().BoolVarP(&listOnly, "list", "l", false, "list the files that would change, and change none")
	rootCmd.AddCommand(fmtCmd)
}

func runFmt(cmd *cobra.Command, args []string) error {
	changed := false
	for _, path := range args {
		if listOnly {
			wouldChange, err := needsFormat(path)
			if err != nil {
				return err
			}
			if wouldChange {
				changed = true
				fmt.Fprintln(cmd.OutOrStdout(), path)
			}
			continue
		}
		wrote, err := slopfmt.FormatFile(path)
		if err != nil {
			return err
		}
		if wrote {
			fmt.Fprintln(cmd.OutOrStdout(), path)
		}
	}
	// -l is a gate: a file that needs formatting fails the run, the way gofmt -l
	// does in CI.
	if listOnly && changed {
		return errFindings
	}
	return nil
}

func needsFormat(path string) (bool, error) {
	findings, err := slopfmt.CheckFile(path)
	if err != nil {
		return false, err
	}
	for _, finding := range findings {
		if finding.Rule == "a paragraph is one line" {
			return true, nil
		}
	}
	return false, nil
}
