package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfmt"
)

// asJSON makes the output machine-readable, which is how a hook consumes it.
var asJSON bool

func init() {
	fix := &cobra.Command{
		Use:   "fix",
		Short: "Repair text on stdin and report what a rewrite cannot repair",
		Long: "fix reads a document on stdin and writes the repaired document on stdout.\n\n" +
			"It joins each hand-wrapped paragraph and cuts the cardinal out of an\n" +
			"inventory count. What no rewrite can repair goes to stderr, and a\n" +
			"remaining finding exits 1.\n\n" +
			"With --json the whole answer is one object on stdout instead, which is\n" +
			"what a PreToolUse hook reads: the repaired text, whether it changed, the\n" +
			"counts removed, and the findings left.",
		Args: cobra.NoArgs,
		RunE: runFix,
	}
	fix.Flags().BoolVar(&asJSON, "json", false, "write the whole answer as one JSON object on stdout")
	rootCmd.AddCommand(fix)
}

func runFix(cmd *cobra.Command, _ []string) error {
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	repair := slopfmt.Fix(string(content))

	if asJSON {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		if err := encoder.Encode(repair); err != nil {
			return err
		}
		return nil
	}

	fmt.Fprint(cmd.OutOrStdout(), repair.Text)
	for _, finding := range repair.Findings {
		fmt.Fprintln(cmd.ErrOrStderr(), finding)
	}
	if len(repair.Findings) > 0 {
		return errFindings
	}
	return nil
}
