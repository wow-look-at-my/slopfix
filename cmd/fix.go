package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfmt"
	"github.com/wow-look-at-my/slopfmt/tombstones"
)

var (
	// asJSON makes the output machine-readable, which is how a hook consumes it.
	asJSON bool
	// fixOnly restricts the run to the rules a caller names.
	fixOnly []string
	// fixPath names the file the text is headed for.
	fixPath string
	// fixMaxLines caps a comment block.
	fixMaxLines int
)

func init() {
	fix := &cobra.Command{
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
		RunE: runFix,
	}
	fix.Flags().BoolVar(&asJSON, "json", false, "write the whole answer as one JSON object on stdout")
	fix.Flags().StringSliceVar(&fixOnly, "only", nil, "apply only these rules: "+strings.Join(ruleNames(), ", "))
	fix.Flags().StringVar(&fixPath, "path", "", "the file the text is headed for")
	fix.Flags().IntVar(&fixMaxLines, "max-comment-lines", tombstones.DefaultMaxCommentLines, "cap a comment block, 0 to turn the cap off")
	rootCmd.AddCommand(fix)
}

func runFix(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fixFiles(cmd, args)
	}
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	repair := slopfmt.Fix(slopfmt.Request{
		Content:         string(content),
		Path:            fixPath,
		Rules:           rules,
		MaxCommentLines: fixMaxLines,
	})

	if asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(repair)
	}

	fmt.Fprint(cmd.OutOrStdout(), repair.Text)
	for _, line := range repair.Removed {
		fmt.Fprintf(cmd.ErrOrStderr(), "removed: %s\n", line)
	}
	for _, hit := range repair.Kept {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: %q\n    %s\n", hit.Tell, hit.Phrase, hit.Line)
	}
	for _, finding := range repair.Findings {
		fmt.Fprintln(cmd.ErrOrStderr(), finding)
	}
	if len(repair.Findings) > 0 || len(repair.Kept) > 0 {
		return errFindings
	}
	return nil
}

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
