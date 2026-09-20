package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "check <file>...",
		Short: "Report what the prose rules reject, and exit 1 when anything does",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runCheck,
	})
}

func runCheck(cmd *cobra.Command, args []string) error {
	found := false
	for _, path := range args {
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
