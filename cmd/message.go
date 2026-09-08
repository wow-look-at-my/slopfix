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

// messageOnly narrows the run to the named rules, since each caller wants its own.
var messageOnly []string

// messageHit is this command's finding, flattened out of the packages so the
// wire shape does not change when a rule moves between them.
type messageHit struct {
	ID string `json:"id"`
	// Tell says in words which shape fired.
	Tell string `json:"tell"`
	// Phrase is the offending wording where the rule can name it. A rule that
	Phrase string `json:"phrase,omitempty"`
	// Sentence quotes the line, for context.
	Sentence string `json:"sentence"`
	Line     int    `json:"line"`
}

// quoted is what a report names: the phrase where the rule found one, and the
// sentence where it did not.
func (h messageHit) quoted() string {
	if h.Phrase != "" {
		return h.Phrase
	}
	return h.Sentence
}

// messageIDs names every rule this command can report, so a typo is rejected
// rather than reading as a clean message.
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
			"reject in it. It exits 1 when anything does, and with --json it writes\n" +
			"the findings as one object on stdout instead.\n\n" +
			"The other commands judge a file. This judges the text the model sends\n" +
			"to the reader, which is never on disk. --only narrows the run to one\n" +
			"rule or one family, because each caller wants its own.",
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
			hits = append(hits, messageHit{
				ID: hit.ID, Tell: hit.Tell, Sentence: hit.Sentence, Line: hit.Line,
			})
		}
	}
	if selected(blamelanguage.ID) {
		for _, hit := range blamelanguage.Check(message) {
			hits = append(hits, messageHit{
				ID: hit.ID, Tell: hit.Tell, Phrase: hit.Phrase, Sentence: hit.Sentence, Line: hit.Line,
			})
		}
	}

	if messageJSON {
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(messageOutput{Findings: hits}); err != nil {
			return err
		}
	} else {
		for _, hit := range hits {
			fmt.Fprintf(cmd.OutOrStdout(), "%d: [%s] %s: %q\n", hit.Line, hit.ID, hit.Tell, hit.quoted())
		}
	}
	if len(hits) > 0 {
		return errFindings
	}
	return nil
}
