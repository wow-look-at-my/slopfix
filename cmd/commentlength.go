package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/commentlength"
)

func init() {
	c := &cobra.Command{
		Use:   "comment-length <path>...",
		Short: "Report a comment longer than the code it documents, and exit 1 when anything is",
		Long: "Reads a real syntax tree, so the span a comment is weighed against is exact.\n" +
			"A directory is walked; a file the rule has no grammar for is skipped.\n\n" +
			"--fix cuts each over-long block back inside its budget, from the end, and\n" +
			"never past the opening sentence.",
		Args: cobra.MinimumNArgs(1),
		RunE: runCommentLength,
	}
	c.Flags().Bool("fix", false, "repair each file in place rather than report it")
	rootCmd.AddCommand(c)
}

func runCommentLength(cmd *cobra.Command, args []string) error {
	repair, err := cmd.Flags().GetBool("fix")
	if err != nil {
		return err
	}
	found := false
	for _, arg := range args {
		paths, err := commentTargets(arg, commentlength.Parsed)
		if err != nil {
			return err
		}
		for _, path := range paths {
			hit, err := lengthOf(cmd, path, repair)
			if err != nil {
				return err
			}
			found = found || hit
		}
	}
	if found {
		return errFindings
	}
	return nil
}

// lengthOf reports or repairs one file, and answers whether anything is left
// for the caller to fail over. A repaired file leaves nothing.
func lengthOf(cmd *cobra.Command, path string, repair bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	hits := commentlength.Check(path, string(src))
	if len(hits) == 0 {
		return false, nil
	}
	if repair {
		out, changed := commentlength.Fix(path, string(src))
		if changed {
			if err := os.WriteFile(path, []byte(out), info.Mode().Perm()); err != nil {
				return false, err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: repaired\n", path)
		}
		// A block the repair cannot shorten is still a finding.
		return len(commentlength.Check(path, out)) > 0, nil
	}
	for _, hit := range hits {
		fmt.Fprintf(cmd.OutOrStdout(), "%s:%d: %s: %s\n", path, hit.Line, hit.Tell, hit.Sentence)
	}
	return true, nil
}
