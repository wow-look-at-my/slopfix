// Package cmd holds the CLI. Each command registers itself from its own file.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// errFindings is the silent sentinel a failing check exits on.
var errFindings = errors.New("findings reported")

var rootCmd = &cobra.Command{
	Use:   "slopfmt",
	Short: "Format and check markdown prose against the org's writing rules",
	Long: "slopfmt is the single filter every document passes through.\n\n" +
		"`fmt` rewrites a file so each paragraph is one line, moving newlines and\n" +
		"nothing else. `check` reports what a rewrite cannot repair: a sentence over\n" +
		"the cap, a contraction, a banned modal, a semicolon, a comma splice.\n\n" +
		"The hooks and CI both shell out to this binary, so all three agree.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI, failing without a usage dump.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if !errors.Is(err, errFindings) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}
