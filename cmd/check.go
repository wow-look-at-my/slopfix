package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// ruleNames answers every category a caller may name to --only.
func ruleNames() []string {
	names := make([]string, 0, len(slopfix.AllRules))
	for _, rule := range slopfix.AllRules {
		names = append(names, string(rule))
	}
	return names
}

// selectedRules turns --only into what Fix takes.
//
// An entry is a category (`ste`) or a rule ID (`ste/semicolon`), the name the
// report prints. An ID turns its category on too. An unknown name is an error,
// because a typo that quietly applies nothing reads as a clean file.
func selectedRules(only []string) ([]slopfix.Rule, []string, error) {
	var rules []slopfix.Rule
	var ids []string
	for _, name := range only {
		name = strings.TrimSpace(name)
		if category, _, isID := strings.Cut(name, "/"); isID {
			rule := slopfix.Rule(category)
			if !slices.Contains(slopfix.AllRules, rule) {
				return nil, nil, fmt.Errorf("unknown rule %q: its category is not one of %s", name, strings.Join(ruleNames(), ", "))
			}
			known := slopfix.IDsFor(rule)
			if !known.Contains(name) {
				return nil, nil, fmt.Errorf("unknown rule %q: %s holds %s", name, category, slopfix.Listed(known))
			}
			rules = append(rules, rule)
			ids = append(ids, name)
			continue
		}
		rule := slopfix.Rule(name)
		if !slices.Contains(slopfix.AllRules, rule) {
			return nil, nil, fmt.Errorf("unknown rule %q: pick from %s, or name one rule as category/rule", name, strings.Join(ruleNames(), ", "))
		}
		rules = append(rules, rule)
	}
	return rules, ids, nil
}

// repairOf answers what every rule makes of a file. Repairing, it writes the
// repair back; reporting, it runs the same rules and leaves the file alone, so
// a rule selection means the same thing either way.
func repairOf(path string, request slopfix.Request, repairing bool) (slopfix.Repair, error) {
	if repairing {
		return slopfix.FixFileWith(path, request)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return slopfix.Repair{}, err
	}
	request.Content, request.Path = string(content), path
	return slopfix.Fix(request), nil
}

var (
	// checkFix asks check to repair what it can rather than only report it.
	checkFix bool
	// checkJSON makes the answer a single object, which is how a hook consumes it.
	checkJSON bool
	// checkOnly restricts the run to the rules a caller names.
	checkOnly []string
	// checkPath names the file text on stdin is headed for.
	checkPath string
	// checkMaxLines caps a comment block.
	checkMaxLines int
)

func init() {
	check := &cobra.Command{
		Use:   "check [--fix] [file]...",
		Short: "Report what every rule rejects, and with --fix repair what it can",
		Long: "check reads each file it is named and reports every finding. With --fix\n" +
			"it repairs each one in place first, and reports what a rewrite cannot\n" +
			"repair. A directory is walked. Walked from a repository root, the repo\n" +
			"rules also judge which markdown files the repository keeps.\n\n" +
			"With no file it reads a document on stdin. Named as fix, or given\n" +
			"--fix, it writes the repaired document on stdout and its findings on\n" +
			"stderr; --json writes the whole answer as one object instead, which is\n" +
			"what a PreToolUse hook reads.",
		// fix is check --fix. A file decides which rules read it, so no other name is needed.
		Aliases: []string{"fix"},
		Args:    cobra.ArbitraryArgs,
		RunE:    runCheck,
	}
	check.Flags().BoolVar(&checkFix, "fix", false, "write the repair back to each file")
	check.Flags().BoolVar(&checkJSON, "json", false, "write the whole answer as one JSON object on stdout")
	check.Flags().StringSliceVar(&checkOnly, "only", nil,
		"run only these, as a comma-separated list. An entry is a category ("+
			strings.Join(ruleNames(), ", ")+") or a single rule ID, which is the name the report prints")
	check.Flags().StringVar(&checkPath, "path", "", "the file the text on stdin is headed for")
	check.Flags().IntVar(&checkMaxLines, "max-comment-lines", tombstones.DefaultMaxCommentLines, "cap a comment block, 0 to turn the cap off")
	rootCmd.AddCommand(check)
}

func runCheck(cmd *cobra.Command, args []string) error {
	// Invoked as "fix", the repair is what was asked for, flag or no flag.
	repairing := checkFix || cmd.CalledAs() == "fix"
	rules, ids, err := selectedRules(checkOnly)
	if err != nil {
		return err
	}
	request := slopfix.Request{
		Path:            checkPath,
		Rules:           rules,
		IDs:             ids,
		MaxCommentLines: checkMaxLines,
	}
	if len(args) == 0 {
		return checkStdin(cmd, request, repairing)
	}
	found := false
	for _, path := range args {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		// A directory is the whole tree under it, which is what a build names.
		if info.IsDir() {
			if treeFindings(cmd, path, request, repairing) {
				found = true
			}
			continue
		}
		repair, err := repairOf(path, request, repairing)
		if err != nil {
			return err
		}
		for _, finding := range repair.Findings {
			found = true
			fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", path, finding)
		}
	}
	if found {
		return errFindings
	}
	return nil
}

// Repairing, each file that changed is named as it is written.
func treeFindings(cmd *cobra.Command, root string, request slopfix.Request, repairing bool) bool {
	walk := slopfix.CheckTreeWith
	if repairing {
		walk = slopfix.FixTreeWith
	}
	out := walk(root, request)
	for _, path := range out.Repaired {
		fmt.Fprintln(cmd.OutOrStdout(), path)
	}
	for _, finding := range out.Findings {
		fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", finding.Path, finding.Finding)
	}
	for _, kept := range out.Kept {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s:%d: [%s] %s: %q\n", kept.Path, kept.LineNo, kept.ID, kept.Tell, kept.Phrase)
	}
	return len(out.Findings) > 0 || len(out.Kept) > 0
}

// checkStdin answers for a document on stdin rather than a named file.
func checkStdin(cmd *cobra.Command, request slopfix.Request, repairing bool) error {
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	request.Content = string(content)
	repair := slopfix.Fix(request)

	if checkJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(repair)
	}
	if repairing {
		fmt.Fprint(cmd.OutOrStdout(), repair.Text)
	}
	for _, line := range repair.Removed {
		fmt.Fprintf(cmd.ErrOrStderr(), "removed: %s\n", line)
	}
	for _, hit := range repair.Kept {
		fmt.Fprintf(cmd.ErrOrStderr(), "[%s] %s: %q\n    %s\n", hit.ID, hit.Tell, hit.Phrase, hit.Line)
	}
	for _, finding := range repair.Findings {
		fmt.Fprintln(cmd.ErrOrStderr(), finding)
	}
	if len(repair.Findings) > 0 || len(repair.Kept) > 0 {
		return errFindings
	}
	return nil
}
