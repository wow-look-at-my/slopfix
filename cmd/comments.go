package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentnumbers"
)

func init() {
	c := &cobra.Command{
		Use:   "comments <path>...",
		Short: "Report a number stated in a comment, and exit 1 when anything does",
		Long: "Reads comments by their delimiters rather than by a grammar, so it answers\n" +
			"for every language it knows and on a tree that does not compile. A directory\n" +
			"is walked; a file is read whatever its extension.\n\n" +
			"--fix says the number in words wherever the table covers it, and cuts the\n" +
			"sentence carrying any number it does not. A cut sentence is printed, because\n" +
			"nothing else tells you what the repair took.",
		Args: cobra.MinimumNArgs(1),
		RunE: runComments,
	}
	c.Flags().Bool("fix", false, "repair each file in place rather than report it")
	rootCmd.AddCommand(c)
}

// skipDirs hold text nobody in the tree authored.
var skipDirs = set.Of("vendor", "node_modules", "testdata", "build")

func runComments(cmd *cobra.Command, args []string) error {
	repair, err := cmd.Flags().GetBool("fix")
	if err != nil {
		return err
	}
	found := false
	for _, arg := range args {
		paths, err := commentTargets(arg, commentnumbers.Supported)
		if err != nil {
			return err
		}
		for _, path := range paths {
			hit, err := numbersOf(cmd, path, repair)
			if err != nil {
				return err
			}
			found = found || hit
		}
	}
	if found {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", commentnumbers.Remedy)
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
	hits := commentnumbers.Check(path, string(src))
	if len(hits) == 0 {
		return false, nil
	}
	if !repair {
		printHits(cmd, path, hits)
		return true, nil
	}
	fixed := commentnumbers.Fix(path, string(src))
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
	left := commentnumbers.Check(path, fixed.Text)
	printHits(cmd, path, left)
	return len(left) > 0, nil
}

// printHits prints a finding per line, the way a compiler names a warning.
func printHits(cmd *cobra.Command, path string, hits []commentnumbers.Hit) {
	for _, hit := range hits {
		fmt.Fprintf(cmd.OutOrStdout(), "%s:%d:%d: %q is a number in a comment\n",
			path, hit.Line, hit.Col, hit.Number)
	}
}

// commentTargets lists what to read under an argument, keeping the files the
// caller's rule reads. A named file is read whatever its extension, because
// naming it is the request.
func commentTargets(arg string, reads func(string) bool) ([]string, error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{arg}, nil
	}
	var out []string
	err = filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != arg && (strings.HasPrefix(d.Name(), ".") || skipDirs.Contains(d.Name()) || isSubmodule(path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if reads(path) {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// isSubmodule reports whether dir is a git submodule's working tree.
func isSubmodule(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.Mode().IsRegular()
}
