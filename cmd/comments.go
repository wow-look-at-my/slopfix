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
	rootCmd.AddCommand(&cobra.Command{
		Use:   "comments <path>...",
		Short: "Report a number stated in a comment, and exit 1 when anything does",
		Long: "Reads comments by their delimiters rather than by a grammar, so it answers\n" +
			"for every language it knows and on a tree that does not compile. A directory\n" +
			"is walked; a file is read whatever its extension.",
		Args: cobra.MinimumNArgs(1),
		RunE: runComments,
	})
}

// skipDirs hold text nobody in the tree authored.
var skipDirs = set.Of("vendor", "node_modules", "testdata", "build")

func runComments(cmd *cobra.Command, args []string) error {
	found := false
	for _, arg := range args {
		paths, err := commentTargets(arg)
		if err != nil {
			return err
		}
		for _, path := range paths {
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, hit := range commentnumbers.Check(path, string(src)) {
				found = true
				fmt.Fprintf(cmd.OutOrStdout(), "%s:%d:%d: %q is a number in a comment\n",
					path, hit.Line, hit.Col, hit.Number)
			}
		}
	}
	if found {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", commentnumbers.Remedy)
		return errFindings
	}
	return nil
}

// commentTargets lists what to read under an argument. A named file is read
// whatever its extension, because naming it is the request.
func commentTargets(arg string) ([]string, error) {
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
			if path != arg && (strings.HasPrefix(d.Name(), ".") || skipDirs.Contains(d.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if commentnumbers.Supported(path) {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}
