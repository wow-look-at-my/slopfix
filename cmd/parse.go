package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// showTags is the --tags flag: print each word's part of speech too.
var showTags bool

func init() {
	parseCmd := &cobra.Command{
		Use:   "parse [sentence]...",
		Short: "Print the phrases and clauses the prose rules read in a sentence",
		Long: "Print the phrases and clauses the prose rules read in each sentence. " +
			"With no argument, the sentence is read from stdin.",
		RunE: runParse,
	}
	parseCmd.Flags().BoolVar(&showTags, "tags", false, "print each word with its part-of-speech tag")
	rootCmd.AddCommand(parseCmd)
}

func runParse(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		in, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		args = []string{strings.TrimSpace(string(in))}
	}
	out := cmd.OutOrStdout()
	for _, text := range args {
		s := syntax.Parse(text, nil)
		if showTags {
			fmt.Fprintln(out, s.Tags())
		}
		fmt.Fprintln(out, s.Brackets())
		fmt.Fprintln(out, s.Outline())
	}
	return nil
}
