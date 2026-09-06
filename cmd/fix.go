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
<<<<<<< HEAD
		Use:   "fix [file]...",
		Short: "Repair a document and report what a rewrite cannot repair",
		Long: "fix repairs each file it is named, in place. With no file it reads a\n" +
			"document on stdin and writes the repaired document on stdout.\n\n" +
			"It joins each hand-wrapped paragraph, cuts the cardinal out of an\n" +
			"inventory count, expands a contraction, writes the approved word for a\n" +
			"banned modal, and turns a semicolon and a comma splice into the period\n" +
			"each stands in for. A sentence over the word cap needs a writer, so it\n" +
			"is reported and left alone.\n\n" +
			"What no rewrite can repair goes to stderr, and a remaining finding\n" +
			"exits 1.\n\n" +
			"With --json the whole answer is one object on stdout instead, which is\n" +
			"what a PreToolUse hook reads: the repaired text, whether it changed, the\n" +
			"counts removed, and the findings left.",
=======
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
>>>>>>> origin/master
		RunE: runFix,
	}
	fix.Flags().BoolVar(&asJSON, "json", false, "write the whole answer as one JSON object on stdout")
	rootCmd.AddCommand(fix)
}

<<<<<<< HEAD
func runFix(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fixFiles(cmd, args)
	}
=======
func runFix(cmd *cobra.Command, _ []string) error {
>>>>>>> origin/master
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
<<<<<<< HEAD

// fixFiles repairs each named file in place. It names the ones it rewrote on
// stdout, and reports what is left on stderr against the file it belongs to.
func fixFiles(cmd *cobra.Command, paths []string) error {
	found := false
	for _, path := range paths {
		repair, err := slopfmt.FixFile(path)
		if err != nil {
			return err
		}
		if repair.Changed {
			fmt.Fprintln(cmd.OutOrStdout(), path)
		}
		for _, finding := range repair.Findings {
			found = true
			fmt.Fprintf(cmd.ErrOrStderr(), "%s:%s\n", path, finding)
		}
	}
	if found {
		return errFindings
	}
	return nil
}
=======
>>>>>>> origin/master
