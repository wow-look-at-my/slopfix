package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfmt/tombstones"
)

var (
	// tombstonesJSON makes the output machine-readable, which is how a hook
	// reads it.
	tombstonesJSON bool
	// tombstonesPath names the file the text is headed for. It decides which
	// comment syntax applies, and whether the text is prose or source.
	tombstonesPath string
	// tombstonesMaxLines caps one comment block. Zero turns the cap off.
	tombstonesMaxLines int
)

func init() {
	command := &cobra.Command{
		Use:   "tombstones",
		Short: "Strip each tombstone comment out of the text on stdin",
		Long: "tombstones reads the text a write adds and returns it with each\n" +
			"tombstone comment removed.\n\n" +
			"A finding that no whole-line deletion resolves is reported instead of\n" +
			"removed. A caller that refuses a write refuses on those.",
		Args: cobra.NoArgs,
		RunE: runTombstones,
	}
	command.Flags().BoolVar(&tombstonesJSON, "json", false, "write the whole answer as one JSON object on stdout")
	command.Flags().StringVar(&tombstonesPath, "path", "", "the file the text is headed for")
	command.Flags().IntVar(&tombstonesMaxLines, "max-lines", tombstones.DefaultMaxCommentLines, "cap one comment block, 0 to turn the cap off")
	rootCmd.AddCommand(command)
}

func runTombstones(cmd *cobra.Command, _ []string) error {
	if tombstonesPath == "" {
		return fmt.Errorf("--path is required: the comment syntax follows the file's name")
	}
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	repair := tombstones.Fix(tombstonesPath, string(content), tombstonesMaxLines)

	if tombstonesJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(repair)
	}
	fmt.Fprint(cmd.OutOrStdout(), repair.Text)
	for _, line := range repair.Removed {
		fmt.Fprintf(cmd.ErrOrStderr(), "removed: %s\n", line)
	}
	for _, hit := range repair.Kept {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: %q\n    %s\n", hit.Tell, hit.Phrase, hit.Line)
	}
	return nil
}
