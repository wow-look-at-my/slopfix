// file.go is the whole file-facing surface: a single command that reports,
// and a single that repairs.
//
// They were commands over the same rules, split by which rule family a caller
// happened to want: prose, comment length, numbers in comments, workflows,
// and paragraph joining. The path already decides which of those read a file,
// so the split asked the caller to know something the binary works out
// anyway, and each was a place a new rule could be left out of.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

var (
	// fileJSON makes the output machine-readable, which is how a hook and a
	// language server consume it.
	fileJSON bool
	// fileOnly restricts the run to the rules a caller names.
	fileOnly []string
	// filePath names the file text on stdin is headed for.
	filePath string
	// fileMaxLines caps a comment block.
	fileMaxLines int
)

func init() {
	parent := &cobra.Command{
		Use:   "file",
		Short: "Report or repair what the rules say about a file",
		Long: "file carries the two things a caller does to a file: check it, or fix\n" +
			"it. Both read every rule, and the path decides which of them apply. A\n" +
			"workflow and an action manifest take the workflow rules, a document takes\n" +
			"the prose rules, and a source file takes the rules that read its\n" +
			"comments.\n\n" +
			"Naming a rule family is what --only is for. It is not what a separate\n" +
			"command was for: a command per family leaves each new rule needing a home,\n" +
			"and the ones that never got one ran nowhere.",
	}

	check := &cobra.Command{
		Use:   "check [file]...",
		Short: "Report what the rules reject, and exit 1 when anything does",
		Long: "check reports each finding against the file it sits in. With no file it\n" +
			"reads one document on stdin, and --path names the file that text is\n" +
			"headed for, which is what decides the rules that read it.\n\n" +
			"With --json the answer is one object per file on stdout, which is what a\n" +
			"language server with an open buffer reads. It still exits 1 on a finding,\n" +
			"so a caller that only wants the verdict needs no parser.",
		RunE: runFileCheck,
	}
	check.Flags().BoolVar(&fileJSON, "json", false, "write the findings as JSON on stdout")
	check.Flags().StringSliceVar(&fileOnly, "only", nil, onlyUsage())
	check.Flags().StringVar(&filePath, "path", "", "the file text on stdin is headed for")

	fix := &cobra.Command{
		Use:   "fix [file]...",
		Short: "Repair a file in place, and report what no rewrite repairs",
		Long: "fix repairs each file it is named, in place, and names the ones it\n" +
			"rewrote. With no file it reads a document on stdin and writes the\n" +
			"repaired document on stdout.\n\n" +
			"It joins each hand-wrapped paragraph, cuts the cardinal out of an\n" +
			"inventory count, expands a contraction, writes the approved word for a\n" +
			"banned modal, turns a semicolon and a comma splice into the period each\n" +
			"stands in for, folds a run of comment lines into the one the rule allows,\n" +
			"and cuts a comment back inside the code it documents.\n\n" +
			"What no rewrite repairs goes to stderr, and a remaining finding exits 1.",
		RunE: runFileFix,
	}
	fix.Flags().BoolVar(&fileJSON, "json", false, "write the whole answer as one JSON object on stdout")
	fix.Flags().StringSliceVar(&fileOnly, "only", nil, onlyUsage())
	fix.Flags().StringVar(&filePath, "path", "", "the file text on stdin is headed for")
	fix.Flags().IntVar(&fileMaxLines, "max-comment-lines", tombstones.DefaultMaxCommentLines,
		"cap a comment block, 0 to turn the cap off")

	parent.AddCommand(check, fix)
	rootCmd.AddCommand(parent)
}

// judged reports whether any rule reads this file, which is what a walk keeps.
func judged(path string) bool {
	return slopfix.IsDocument(path) || commentfix.Supported(path) || workflow.Judges(path)
}

// targets lists what to read under an argument. A directory takes the library's
// walk, so a caller embedding it skips the same text. A named file is read
// whatever its extension: naming it is the request.
func targets(args []string) ([]string, error) {
	var out []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, arg)
			continue
		}
		out = append(out, commentfix.TreeFilesMatching(arg, judged)...)
	}
	return out, nil
}

func onlyUsage() string {
	return "run only these, as a comma-separated list. An entry is a category (" +
		strings.Join(ruleNames(), ", ") + ") or a single rule ID, which is the name the report prints"
}

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

// reportFinding is the wire contract, held apart from ste.Finding so a rename
// cannot change it.
type reportFinding struct {
	// ID names the rule, the way a compiler names a warning.
	ID string `json:"id"`
	// Line is where the finding starts.
	Line    int `json:"line"`
	EndLine int `json:"endLine"`
	// Rule says what the ID stands for. Detail quotes the text, and Fix names the repair.
	Rule   string `json:"rule"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
	// Repairable reports whether slopfix repairs this defect itself.
	Repairable bool `json:"repairable"`
}

type reportOutput struct {
	// Path echoes what was judged, so a caller batching calls tells them apart.
	Path     string          `json:"path"`
	Findings []reportFinding `json:"findings"`
}

func runFileCheck(cmd *cobra.Command, args []string) error {
	keeps, err := reportFilter(fileOnly)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		if filePath == "" {
			return fmt.Errorf("--path is required when the text arrives on stdin: the path decides which rules read it")
		}
		content, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		return emitFindings(cmd, filePath, slopfix.CheckContent(filePath, string(content)), keeps)
	}

	paths, err := targets(args)
	if err != nil {
		return err
	}
	found := false
	for _, path := range paths {
		findings, err := slopfix.CheckFile(path)
		if err != nil {
			return err
		}
		if err := emitFindings(cmd, path, findings, keeps); err != nil {
			if err != errFindings {
				return err
			}
			found = true
		}
	}
	if found {
		return errFindings
	}
	return nil
}

// emitFindings prints a single file's findings and reports whether any
// survived the caller's selection.
func emitFindings(cmd *cobra.Command, path string, findings []ste.Finding, keeps func(string) bool) error {
	// A nil slice marshals to null, which crashes a caller reading its length.
	out := reportOutput{Path: path, Findings: []reportFinding{}}
	for _, finding := range findings {
		if !keeps(finding.ID) {
			continue
		}
		out.Findings = append(out.Findings, wireFinding(finding))
	}

	if fileJSON {
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
			return err
		}
	} else {
		for _, finding := range findings {
			if keeps(finding.ID) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", path, finding)
			}
		}
	}
	if len(out.Findings) > 0 {
		return errFindings
	}
	return nil
}

func wireFinding(finding ste.Finding) reportFinding {
	end := finding.EndLine
	if end < finding.Line {
		end = finding.Line
	}
	return reportFinding{
		ID:         finding.ID,
		Line:       finding.Line,
		EndLine:    end,
		Rule:       finding.Rule,
		Detail:     finding.Detail,
		Fix:        finding.Fix,
		Repairable: slopfix.Repairable(finding.ID),
	}
}

// reportFilter turns --only into a predicate over a rule ID. An empty list
// reads as every rule.
//
// An unknown name is an error rather than a silent no-op, because a run that
// reports nothing reads exactly like a clean file.
func reportFilter(only []string) (func(string) bool, error) {
	if len(only) == 0 {
		return func(string) bool { return true }, nil
	}
	known := slopfix.AllIDs()
	wanted := set.New[string]()
	for _, name := range only {
		name = strings.TrimSpace(name)
		if !known.Contains(name) {
			return nil, fmt.Errorf("unknown rule %q: pick from %s", name, slopfix.Listed(known))
		}
		wanted.Add(name)
	}
	return wanted.Contains, nil
}

func runFileFix(cmd *cobra.Command, args []string) error {
	rules, ids, err := selectedRules(fileOnly)
	if err != nil {
		return err
	}
	if len(args) > 0 {
		paths, err := targets(args)
		if err != nil {
			return err
		}
		return fixFiles(cmd, paths, rules, ids)
	}

	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	repair := slopfix.Fix(slopfix.Request{
		Content:         string(content),
		Path:            filePath,
		Rules:           rules,
		IDs:             ids,
		MaxCommentLines: fileMaxLines,
	})

	if fileJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(repair)
	}

	fmt.Fprint(cmd.OutOrStdout(), repair.Text)
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

// fixFiles repairs each named file in place, printing every path it changes.
func fixFiles(cmd *cobra.Command, paths []string, rules []slopfix.Rule, ids []string) error {
	found := false
	for _, path := range paths {
		repair, err := slopfix.FixFileWith(path, slopfix.Request{
			Rules:           rules,
			IDs:             ids,
			MaxCommentLines: fileMaxLines,
		})
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
