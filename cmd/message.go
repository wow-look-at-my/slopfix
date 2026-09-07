package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/laziness"
)

// messageJSON writes the whole answer as an object, which is what a hook reads.
var messageJSON bool

func init() {
	command := &cobra.Command{
		Use:   "message",
		Short: "Report what the message rules reject, reading the message on stdin",
		Long: "message reads a closing message on stdin and reports what the rules\n" +
			"reject in it. It exits 1 when anything does.\n\n" +
			"The other commands judge a file. This judges the text the model sends\n" +
			"to the reader, which is never on disk. A Stop hook reads the exit code\n" +
			"and answers a finding with the word the reader would have typed back.\n\n" +
			"With --json the findings are one object on stdout instead.",
		Args: cobra.NoArgs,
		RunE: runMessage,
	}
	command.Flags().BoolVar(&messageJSON, "json", false, "write the findings as one JSON object on stdout")
	rootCmd.AddCommand(command)
}

// messageOutput is this command's wire contract, held apart from the library.
type messageOutput struct {
	Findings []laziness.Hit `json:"findings"`
}

func runMessage(cmd *cobra.Command, _ []string) error {
	text, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	hits := laziness.Check(string(text))

	if messageJSON {
		// A nil slice marshals to null, which breaks a caller reading its length.
		out := messageOutput{Findings: []laziness.Hit{}}
		out.Findings = append(out.Findings, hits...)
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
			return err
		}
	} else {
		for _, hit := range hits {
			fmt.Fprintf(cmd.OutOrStdout(), "%d: [%s] %s: %q\n", hit.Line, hit.ID, hit.Tell, hit.Sentence)
		}
	}
	if len(hits) > 0 {
		return errFindings
	}
	return nil
}
