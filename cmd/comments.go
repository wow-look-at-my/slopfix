package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/commentfix"
)

func init() {
	c := &cobra.Command{
		Use:   "comments <path>...",
		Short: "Report what a comment gets wrong, and exit 1 when anything does",
		Long: "Every comment rule reads one syntax tree: a number a comment states, a block\n" +
			"longer than the code it documents, and a comment that stops mid-thought. A\n" +
			"directory is walked; a file no grammar parses is skipped.\n\n" +
			"--fix says the number in words wherever the table covers it, cuts the sentence\n" +
			"carrying any number it does not, closes a comment left unfinished, and brings\n" +
			"an over-long block back inside its budget. A cut sentence is printed, because\n" +
			"nothing else tells you what the repair took.",
		Args: cobra.MinimumNArgs(1),
		RunE: runComments,
	}
	c.Flags().Bool("fix", false, "repair each file in place rather than report it")
	rootCmd.AddCommand(c)
}

func runComments(cmd *cobra.Command, args []string) error {
	repair, err := cmd.Flags().GetBool("fix")
	if err != nil {
		return err
	}
	found := false
	for _, arg := range args {
		paths, err := commentTargets(arg, commentfix.Supported)
		if err != nil {
			return err
		}
		for _, path := range paths {
			hit, err := numbersOf(cmd, path, repair)
			if err != nil {
				return err
			}
			found = found || hit
			// The length rule needs a grammar, which the number rule does not.
			if !commentfix.Parsed(path) {
				continue
			}
			long, err := lengthOf(cmd, path, repair)
			if err != nil {
				return err
			}
			found = found || long
		}
	}
	if found {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", commentfix.Remedy)
		return errFindings
	}
	return nil
}

// numbersOf reports or repairs a file, and answers whether anything is left for
// the caller to fail over. A repaired file leaves nothing.
func numbersOf(cmd *cobra.Command, path string, repair bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	hits := commentfix.Check(path, string(src))
	if len(hits) == 0 {
		return false, nil
	}
	if !repair {
		printHits(cmd, path, hits)
		return true, nil
	}
	fixed := commentfix.Fix(path, string(src))
	if fixed.Changed {
		if err := os.WriteFile(path, []byte(fixed.Text), info.Mode().Perm()); err != nil {
			return false, err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: repaired\n", path)
	}
	// A cut sentence is gone from the file, so this is the only record of it.
	for _, sentence := range fixed.Removed {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: cut: %s\n", path, sentence)
	}
	left := commentfix.Check(path, fixed.Text)
	printHits(cmd, path, left)
	return len(left) > 0, nil
}

// lengthOf reports or repairs the blocks that outrun the code they document.
func lengthOf(cmd *cobra.Command, path string, repair bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	hits := commentfix.CheckLength(path, string(src))
	if len(hits) == 0 {
		return false, nil
	}
	if repair {
		out, changed := commentfix.FixLength(path, string(src))
		if changed {
			if err := os.WriteFile(path, []byte(out), info.Mode().Perm()); err != nil {
				return false, err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: repaired\n", path)
		}
		// A block the repair cannot shorten is still a finding.
		return len(commentfix.CheckLength(path, out)) > 0, nil
	}
	for _, hit := range hits {
		fmt.Fprintf(cmd.OutOrStdout(), "%s:%d: %s: %s\n", path, hit.Line, hit.Tell, hit.Sentence)
	}
	return true, nil
}

// printHits prints a finding per line, the way a compiler names a warning.
func printHits(cmd *cobra.Command, path string, hits []commentfix.Hit) {
	for _, hit := range hits {
		fmt.Fprintf(cmd.OutOrStdout(), "%s:%d:%d: %q is a number in a comment\n",
			path, hit.Line, hit.Col, hit.Number)
	}
}

// commentTargets lists what to read under an argument. A directory takes the
// library's walk, so a caller embedding it skips the same text. A named file is
// read whatever its extension: naming it is the request.
func commentTargets(arg string, reads func(string) bool) ([]string, error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{arg}, nil
	}
	return commentfix.TreeFilesMatching(arg, reads), nil
}
