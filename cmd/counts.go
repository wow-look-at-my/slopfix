package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfmt/counts"
)

// countsJSON makes the output machine-readable, which is how a hook reads it.
var countsJSON bool

// countsRepair is what `counts` answers with. It is the counts-only shape of
// slopfmt.Repair, so a caller that wants the wrap join asks `fix` instead.
type countsRepair struct {
	Text    string   `json:"text"`
	Changed bool     `json:"changed"`
	Removed []string `json:"removed,omitempty"`
	Lines   []string `json:"lines,omitempty"`
}

func init() {
	command := &cobra.Command{
		Use:   "counts",
		Short: "Cut the cardinal out of every inventory count on stdin",
		Long: "counts reads a document on stdin and writes it back with the cardinal\n" +
			"removed from each inventory count.\n\n" +
			"It touches nothing else. A caller that also wants paragraphs joined and\n" +
			"the STE rules reported asks `fix` instead.",
		Args: cobra.NoArgs,
		RunE: runCounts,
	}
	command.Flags().BoolVar(&countsJSON, "json", false, "write the whole answer as one JSON object on stdout")
	rootCmd.AddCommand(command)
}

func runCounts(cmd *cobra.Command, _ []string) error {
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	text, cut := counts.Strip(string(content))
	repair := countsRepair{Text: text, Changed: text != string(content)}
	for _, hit := range cut {
		repair.Removed = append(repair.Removed, hit.Phrase)
		repair.Lines = append(repair.Lines, hit.Line)
	}

	if countsJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(repair)
	}
	fmt.Fprint(cmd.OutOrStdout(), repair.Text)
	for i, phrase := range repair.Removed {
		fmt.Fprintf(cmd.ErrOrStderr(), "removed %q from: %s\n", phrase, repair.Lines[i])
	}
	return nil
}
