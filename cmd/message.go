package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/laziness"
)

// messageJSON writes the whole answer as an object, which is what a hook reads.
var messageJSON bool

// messageOnly narrows the run to the named rules. The stop refusal wants the
// punt and the display annotation wants the deflection, so a command serving
// both has to be selectable.
var messageOnly []string

// messageHit is this command's finding, flattened out of the packages so the
// wire shape does not change when a rule moves between them.
type messageHit struct {
	ID       string `json:"id"`
	Tell     string `json:"tell"`
	Sentence string `json:"sentence"`
	Line     int    `json:"line"`
}

// messageIDs names every rule this command can report, so a typo is rejected
// rather than silently selecting nothing and reading as a clean message.
func messageIDs() set.Set[string] {
	return set.Of(laziness.ID, blamelanguage.ID)
}

// family is the part of a rule ID before the slash, which a caller writes to
// select everything under that heading.
func family(id string) string {
	before, _, _ := strings.Cut(id, "/")
	return before
}

// selected answers whether a rule runs. An empty --only means every rule.
func selected(id string) bool {
	if len(messageOnly) == 0 {
		return true
	}
	return slices.Contains(messageOnly, id) || slices.Contains(messageOnly, family(id))
}

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
	command.Flags().StringSliceVar(&messageOnly, "only", nil,
		"run only these rules, or these families ("+slopfix.Listed(messageIDs())+")")
	rootCmd.AddCommand(command)
}

// messageOutput is this command's wire contract, held apart from the library.
type messageOutput struct {
	Findings []messageHit `json:"findings"`
}

func runMessage(cmd *cobra.Command, _ []string) error {
	known := messageIDs()
	for _, want := range messageOnly {
		if known.Contains(want) {
			continue
		}
		if slices.ContainsFunc(slices.Collect(known.All()), func(id string) bool { return family(id) == want }) {
			continue
		}
		return fmt.Errorf("unknown rule %q: choose from %s", want, slopfix.Listed(known))
	}

	text, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	message := string(text)

	// A nil slice marshals to null, which breaks a caller reading its length.
	hits := []messageHit{}
	if selected(laziness.ID) {
		for _, hit := range laziness.Check(message) {
			hits = append(hits, messageHit(hit))
		}
	}
	if selected(blamelanguage.ID) {
		for _, hit := range blamelanguage.Check(message) {
			hits = append(hits, messageHit(hit))
		}
	}

	if messageJSON {
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(messageOutput{Findings: hits}); err != nil {
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
