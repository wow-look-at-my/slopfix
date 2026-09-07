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
	"github.com/wow-look-at-my/slopfix/ste"
)

var (
	reportPath string
	reportOnly []string
)

func init() {
	command := &cobra.Command{
		Use:   "report",
		Short: "Report the findings in one document on stdin, as JSON",
		Long: "report reads one document on stdin and writes its findings on stdout as\n" +
			"JSON. --path names the file the text is headed for, which decides whether\n" +
			"the workflow rules or the prose rules read it.\n\n" +
			"check and workflows read files and print for a person. This answers a\n" +
			"program holding text that is not on disk yet: a language server with an\n" +
			"open buffer, or a hook judging the text a write adds. Every one of them\n" +
			"gets the verdict this binary gives CI, rather than a second copy of the\n" +
			"rules that is correct on the day somebody writes it.\n\n" +
			"It always exits 0. A finding is the answer, not a failure: the caller\n" +
			"decides what a finding means.",
		Args: cobra.NoArgs,
		RunE: runReport,
	}
	command.Flags().StringVar(&reportPath, "path", "",
		"the file the text is headed for. Required: the path decides which rules read it")
	command.Flags().StringSliceVar(&reportOnly, "only", nil,
		"report only these rule IDs, as a comma-separated list. Defaults to every rule: "+
			strings.Join(slopfix.AllIDs(), ", "))
	rootCmd.AddCommand(command)
}

// reportFinding is this command's wire contract, named apart from ste.Finding
// so a field the library renames cannot silently change what a caller reads.
type reportFinding struct {
	// ID names the rule, the way a compiler names a warning.
	ID string `json:"id"`
	// Line is where the finding starts, counting from the top of the file.
	// EndLine repeats it for a finding that covers a single line.
	Line    int `json:"line"`
	EndLine int `json:"endLine"`
	// Rule says what the ID stands for, Detail quotes the text, and Fix names
	// the repair.
	Rule   string `json:"rule"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

type reportOutput struct {
	// Path echoes what was judged, so a caller batching calls tells them apart.
	Path     string          `json:"path"`
	Findings []reportFinding `json:"findings"`
}

func runReport(cmd *cobra.Command, _ []string) error {
	return reportContent(cmd, reportPath, reportOnly)
}

// reportContent takes its selection as arguments rather than reading the flag
// globals, so a test never swaps state a parallel sibling is reading.
func reportContent(cmd *cobra.Command, path string, only []string) error {
	if path == "" {
		return fmt.Errorf("--path is required: the path decides whether the workflow rules or the prose rules read the text")
	}
	keeps, err := reportFilter(only)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}

	// A nil slice marshals to null, which crashes a caller reading its length.
	out := reportOutput{Path: path, Findings: []reportFinding{}}
	for _, finding := range slopfix.CheckContent(path, string(content)) {
		if !keeps(finding.ID) {
			continue
		}
		out.Findings = append(out.Findings, wireFinding(finding))
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
}

func wireFinding(finding ste.Finding) reportFinding {
	end := finding.EndLine
	if end < finding.Line {
		end = finding.Line
	}
	return reportFinding{
		ID:      finding.ID,
		Line:    finding.Line,
		EndLine: end,
		Rule:    finding.Rule,
		Detail:  finding.Detail,
		Fix:     finding.Fix,
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
		if !slices.Contains(known, name) {
			return nil, fmt.Errorf("unknown rule %q: pick from %s", name, strings.Join(known, ", "))
		}
		wanted.Add(name)
	}
	return wanted.Contains, nil
}
