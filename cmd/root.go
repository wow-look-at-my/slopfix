// Package cmd holds the CLI. Each command registers itself from its own file.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/timing"
)

var errFindings = errors.New("findings reported")

var rootCmd = &cobra.Command{
	Use:   "slopfix",
	Short: "Format and check markdown prose against the org's writing rules",
	Long: "slopfix is the single filter every document passes through.\n\n" +
		"`fmt` rewrites a file so each paragraph is one line, moving newlines and\n" +
		"nothing else. `check` reports what a rewrite cannot repair: a sentence over\n" +
		"the cap, a contraction, a banned modal, a semicolon, a comma splice.\n\n" +
		"The hooks and CI both shell out to this binary, so all three agree.",
	SilenceUsage:      true,
	SilenceErrors:     true,
	PersistentPreRunE: startTiming,
}

func init() {
	rootCmd.PersistentFlags().Bool("timing", false,
		"print how long each rule and each parse took, slowest first, on stderr. "+
			"The "+timing.EnvVar+" environment variable does the same for a hook")
}

// startTiming switches recording on when the flag asks, before the first phase
// opens. The environment variable is read by the timing package itself.
func startTiming(cmd *cobra.Command, _ []string) error {
	on, err := cmd.Flags().GetBool("timing")
	if err != nil {
		return err
	}
	if on {
		timing.Enable()
	}
	return nil
}

// Execute runs the CLI, failing without a usage dump.
//
// The breakdown goes to stderr after the command, whatever the command
// answered, so a run that ends in findings still reports where its time went
// and no caller parsing stdout sees an extra word.
func Execute() {
	err := rootCmd.Execute()
	timing.Report(os.Stderr)
	if err != nil {
		if !errors.Is(err, errFindings) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}
