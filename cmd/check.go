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
	"github.com/wow-look-at-my/slopfix/forkscope"
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
// An entry is a category (`ste`) or a rule ID (`english/semicolon`), the name the
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
// a rule selection means the same thing either way. In a fork, a file the fork
// never touched answers nothing, and only the fork's lines change or count.
func repairOf(forks forkscope.Resolver, path string, request slopfix.Request, repairing bool) (slopfix.Repair, error) {
	if repairing {
		return slopfix.FixFileIn(forks, path, request)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return slopfix.Repair{}, err
	}
	scope, err := slopfix.FileScope(forks, path, string(content))
	if err != nil {
		return slopfix.Repair{}, err
	}
	if scope != nil && scope.Empty() {
		return slopfix.Repair{Text: string(content)}, nil
	}
	request.Content, request.Path, request.Owned = string(content), path, scope
	repair := slopfix.Report(request)
	if len(repair.Unmet) > 0 {
		return repair, &slopfix.UnmetError{Path: path, Unmet: repair.Unmet}
	}
	return repair, nil
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
	// checkMessage reads stdin as a closing message rather than as a file.
	checkMessage bool
	// checkCmd is check, which the kinds of check that read no rule register under.
	checkCmd *cobra.Command
)

func init() {
	check := &cobra.Command{
		Use:   "check [--fix] [file]...",
		Short: "Report what every rule rejects, and with --fix repair what it can",
		Long: "check reads each file it is named and reports every finding. Every rule\n" +
			"runs unless --only names a subset. The path decides which rules read a\n" +
			"file. With --fix it repairs each file in place first, and reports what a\n" +
			"rewrite cannot repair. A directory is walked. Walked from a repository\n" +
			"root, the repo rules also judge which markdown files the repository keeps.\n\n" +
			"With no file it reads text on stdin, headed for --path. Named as fix, or\n" +
			"given --fix, it writes the repaired text on stdout and its findings on\n" +
			"stderr. --json writes the whole answer as one object instead, and exits 0\n" +
			"on a finding, because the caller decides what a finding means.\n\n" +
			"--message reads stdin as a closing message, which is never on disk, and\n" +
			"judges it with the message rules (" + slopfix.Listed(messageIDs()) + ").",
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
	check.Flags().BoolVar(&checkMessage, "message", false, "judge stdin as a closing message, with the message rules")
	rootCmd.AddCommand(check)
	checkCmd = check
}

func runCheck(cmd *cobra.Command, args []string) error {
	if checkMessage {
		if len(args) > 0 || checkFix || cmd.CalledAs() == "fix" {
			return fmt.Errorf("--message judges stdin and repairs nothing, so it takes no file and no --fix")
		}
		return checkMessageStdin(cmd, checkOnly, checkJSON)
	}
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
		return checkStdin(cmd, request, repairing, checkJSON)
	}
	found := false
	for _, path := range args {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		// A directory is the whole tree under it, which is what a build names.
		if info.IsDir() {
			failed, err := treeFindings(cmd, path, request, repairing, forkscope.Resolver{})
			if err != nil {
				return err
			}
			found = found || failed
			continue
		}
		repair, err := repairOf(forkscope.Resolver{}, path, request, repairing)
		if err != nil {
			return err
		}
		for _, finding := range repair.Findings {
			found = found || !finding.Warning()
			fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", path, finding)
		}
		for _, kept := range repair.Kept {
			found = true
			fmt.Fprintf(cmd.OutOrStdout(), "%s:%d: [%s] %s: %q\n", path, kept.LineNo, kept.ID, kept.Tell, kept.Phrase)
		}
	}
	if found {
		return errFindings
	}
	return nil
}

// Repairing, each file that changed is named as it is written. In a fork, a
// file the fork never touched is neither read nor written, and only the lines
// the fork wrote can change or fail. The resolver comes in as an argument, so
// a test never sets the process environment.
func treeFindings(cmd *cobra.Command, root string, request slopfix.Request, repairing bool, forks forkscope.Resolver) (bool, error) {
	own, err := forks.Lines(root)
	if err != nil {
		return false, err
	}
	request.Fork = own
	walk := slopfix.CheckTreeWith
	if repairing {
		walk = slopfix.FixTreeWith
	}
	out := walk(root, request).Within(own, root)
	for _, path := range out.Repaired {
		fmt.Fprintln(cmd.OutOrStdout(), path)
	}
	failed := false
	for _, finding := range out.Findings {
		failed = failed || !finding.Finding.Warning()
		fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", finding.Path, finding.Finding)
	}
	for _, kept := range out.Kept {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s:%d: [%s] %s: %q\n", kept.Path, kept.LineNo, kept.ID, kept.Tell, kept.Phrase)
	}
	for _, unmet := range out.Unmet {
		fmt.Fprintln(cmd.ErrOrStderr(), unmet.Error())
	}
	return failed || len(out.Kept) > 0 || len(out.Unmet) > 0, nil
}

// checkStdin answers for text on stdin rather than a named file. It takes the
// JSON choice as an argument, so a test never swaps a parallel sibling's state.
func checkStdin(cmd *cobra.Command, request slopfix.Request, repairing, asJSON bool) error {
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	request.Content = string(content)
	run := slopfix.Report
	if repairing {
		run = slopfix.Fix
	}
	repair := run(request)

	if len(repair.Unmet) > 0 {
		return &slopfix.UnmetError{Path: request.Path, Unmet: repair.Unmet}
	}
	if asJSON {
		out := reportOutput{Path: request.Path, Findings: wireFindings(repair.Findings, repair.Kept)}
		if repairing {
			out.Text, out.Removed = &repair.Text, repair.Removed
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
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
	failed := false
	for _, finding := range repair.Findings {
		failed = failed || !finding.Warning()
		fmt.Fprintln(cmd.ErrOrStderr(), finding)
	}
	if failed || len(repair.Kept) > 0 {
		return errFindings
	}
	return nil
}

// checkMessageStdin judges a closing message on stdin. It takes its selection
// as arguments, so a test never swaps state that a parallel sibling reads.
func checkMessageStdin(cmd *cobra.Command, only []string, asJSON bool) error {
	runs, err := messageSelection(only)
	if err != nil {
		return err
	}
	text, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	findings := messageFindings(string(text), runs)
	if asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(reportOutput{Findings: wireFindings(findings, nil)})
	}
	for _, finding := range findings {
		fmt.Fprintln(cmd.OutOrStdout(), finding)
	}
	if len(findings) > 0 {
		return errFindings
	}
	return nil
}
