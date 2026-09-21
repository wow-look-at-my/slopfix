// Package cmd holds the CLI. Each command registers itself from its own file.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
<<<<<<< HEAD
	"github.com/wow-look-at-my/slopfix/trace"
=======
	"github.com/wow-look-at-my/slopfix/timing"
>>>>>>> 621ca5c6b033fb5794616ec2d8f97519570cf67d
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
<<<<<<< HEAD
	PersistentPreRunE: startTrace,
}

func init() {
	rootCmd.PersistentFlags().Bool("trace", false,
		"print how long each rule and each parse took, slowest first, on stderr. "+
			"The "+trace.EnvVar+" environment variable does the same for a hook")
}

// startTrace switches tracing on when the flag asks, before the earliest
// phase opens. The environment variable is read by the trace package itself.
func startTrace(cmd *cobra.Command, _ []string) error {
	on, err := cmd.Flags().GetBool("trace")
=======
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
>>>>>>> 621ca5c6b033fb5794616ec2d8f97519570cf67d
	if err != nil {
		return err
	}
	if on {
<<<<<<< HEAD
		trace.Enable()
=======
		timing.Enable()
>>>>>>> 621ca5c6b033fb5794616ec2d8f97519570cf67d
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
<<<<<<< HEAD
	trace.Report()
=======
	timing.Report(os.Stderr)
>>>>>>> 621ca5c6b033fb5794616ec2d8f97519570cf67d
	if err != nil {
		if !errors.Is(err, errFindings) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}
