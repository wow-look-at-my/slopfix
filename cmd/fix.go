package cmd

import (
	"encoding/json"
	"fmt"
	"io"
<<<<<<< HEAD
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
=======

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfmt"
)

// asJSON makes the output machine-readable, which is how a hook consumes it.
var asJSON bool
>>>>>>> origin/master

func init() {
	fix := &cobra.Command{
		Use:   "fix",
		Short: "Repair text on stdin and report what a rewrite cannot repair",
<<<<<<< HEAD
		Long: "fix reads text on stdin and writes the repaired text on stdout.\n\n" +
			"It strips a tombstone comment, cuts the cardinal out of an inventory\n" +
			"count, joins each hand-wrapped paragraph, and reports what fails the\n" +
			"merge gate. What no rewrite can repair goes to stderr, and a remaining\n" +
			"finding exits 1.\n\n" +
			"--only names the rules to apply, so a caller that wants a single rule\n" +
			"asks for it here: --only counts, --only tombstones.\n\n" +
			"--path names the file the text is headed for. It decides the comment\n" +
			"syntax, and the tombstone rule needs it. Without it the text is read as\n" +
			"prose.\n\n" +
			"With --json the whole answer is one object on stdout instead, which is\n" +
			"what a PreToolUse hook reads.",
=======
		Long: "fix reads a document on stdin and writes the repaired document on stdout.\n\n" +
			"It joins each hand-wrapped paragraph and cuts the cardinal out of an\n" +
			"inventory count. What no rewrite can repair goes to stderr, and a\n" +
			"remaining finding exits 1.\n\n" +
			"With --json the whole answer is one object on stdout instead, which is\n" +
			"what a PreToolUse hook reads: the repaired text, whether it changed, the\n" +
			"counts removed, and the findings left.",
>>>>>>> origin/master
		Args: cobra.NoArgs,
		RunE: runFix,
	}
	fix.Flags().BoolVar(&asJSON, "json", false, "write the whole answer as one JSON object on stdout")
<<<<<<< HEAD
	fix.Flags().StringSliceVar(&fixOnly, "only", nil, "apply only these rules: "+strings.Join(ruleNames(), ", "))
	fix.Flags().StringVar(&fixPath, "path", "", "the file the text is headed for")
	fix.Flags().IntVar(&fixMaxLines, "max-comment-lines", tombstones.DefaultMaxCommentLines, "cap one comment block, 0 to turn the cap off")
	rootCmd.AddCommand(fix)
}

func ruleNames() []string {
	names := make([]string, 0, len(slopfmt.AllRules))
	for _, rule := range slopfmt.AllRules {
		names = append(names, string(rule))
	}
	return names
}

// selectedRules turns --only into the rules Fix takes. An unknown name is an
// error rather than a silent no-op, because a typo that quietly applies nothing
// reads as a clean file.
func selectedRules(only []string) ([]slopfmt.Rule, error) {
	var rules []slopfmt.Rule
	for _, name := range only {
		rule := slopfmt.Rule(strings.TrimSpace(name))
		if !slices.Contains(slopfmt.AllRules, rule) {
			return nil, fmt.Errorf("unknown rule %q: pick from %s", name, strings.Join(ruleNames(), ", "))
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func runFix(cmd *cobra.Command, _ []string) error {
	rules, err := selectedRules(fixOnly)
	if err != nil {
		return err
	}
=======
	rootCmd.AddCommand(fix)
}

func runFix(cmd *cobra.Command, _ []string) error {
>>>>>>> origin/master
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
<<<<<<< HEAD
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
=======
	repair := slopfmt.Fix(string(content))

	if asJSON {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		if err := encoder.Encode(repair); err != nil {
			return err
		}
		return nil
	}

	fmt.Fprint(cmd.OutOrStdout(), repair.Text)
	for _, finding := range repair.Findings {
		fmt.Fprintln(cmd.ErrOrStderr(), finding)
	}
	if len(repair.Findings) > 0 {
>>>>>>> origin/master
		return errFindings
	}
	return nil
}
