package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/workflow"
)

var (
	workflowExcludes []string
	workflowOnly     []string
)

func init() {
	command := &cobra.Command{
		Use:   "workflows <path>...",
		Short: "Report what the workflow rules reject, and exit 1 when anything does",
		Long: "Walks a directory for every workflow and every action manifest under it, and\n" +
			"reads each by the workflow rules. A named file is read whatever its path.\n\n" +
			"A run that selects no file is a failure, not a pass. The check that scanned\n" +
			"nothing enforced nothing, and a green run there says the opposite.",
		Args: cobra.MinimumNArgs(1),
		RunE: runWorkflows,
	}
	command.Flags().StringSliceVar(&workflowExcludes, "exclude", nil,
		"glob patterns for paths the walk skips, for a fixture that breaks a rule on purpose")
	command.Flags().StringSliceVar(&workflowOnly, "only", nil,
		"report only these rule IDs, as a comma-separated list. Defaults to every rule: "+
			strings.Join(workflow.AllIDs, ", "))
	rootCmd.AddCommand(command)
}

func runWorkflows(cmd *cobra.Command, args []string) error {
	excluded, err := excludeMatcher(workflowExcludes)
	if err != nil {
		return err
	}
	reports, err := selectedIDs(workflowOnly)
	if err != nil {
		return err
	}

	var scanned []string
	found := false
	for _, arg := range args {
		paths, err := workflowTargets(arg)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if excluded(path) {
				continue
			}
			scanned = append(scanned, path)
			findings, err := slopfix.CheckFile(path)
			if err != nil {
				return err
			}
			for _, finding := range findings {
				if !reports(finding.ID) {
					continue
				}
				found = true
				fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", path, finding)
			}
		}
	}

	if len(scanned) == 0 {
		return fmt.Errorf("no workflow and no action manifest under %s, so this run enforced nothing"+
			" -- check the repository out before the step that reads it", strings.Join(args, ", "))
	}
	if found {
		return errFindings
	}
	fmt.Fprintf(cmd.OutOrStdout(), "OK -- %d file(s) read: %s\n", len(scanned), strings.Join(scanned, ", "))
	return nil
}

// workflowTargets lists what to read under an argument. A named file is read
// whatever its path, because naming it is the request.
//
// The walk keeps .github, which every other walk in this package skips as a
// hidden directory. Skipping it here would drop every workflow in the tree.
func workflowTargets(arg string) ([]string, error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{arg}, nil
	}
	var out []string
	err = filepath.WalkDir(arg, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == arg || entry.Name() == ".github" {
				return nil
			}
			if strings.HasPrefix(entry.Name(), ".") || skipDirs.Contains(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if workflow.Judges(path) {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// selectedIDs turns --only into a predicate over a finding's rule ID. An empty
// list reads as every rule.
//
// An unknown name is an error rather than a silent no-op, because a run that
// reports nothing reads exactly like a clean tree.
func selectedIDs(only []string) (func(string) bool, error) {
	if len(only) == 0 {
		return func(string) bool { return true }, nil
	}
	wanted := set.New[string]()
	for _, name := range only {
		name = strings.TrimSpace(name)
		if !slices.Contains(workflow.AllIDs, name) {
			return nil, fmt.Errorf("unknown rule %q: pick from %s",
				name, strings.Join(workflow.AllIDs, ", "))
		}
		wanted.Add(name)
	}
	return func(id string) bool { return wanted.Contains(id) }, nil
}

// excludeMatcher reports whether a path matches any pattern. `**` crosses a
// separator and `*` does not, which is the spelling a workflow author writes.
func excludeMatcher(patterns []string) (func(string) bool, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		expression, err := globToRegexp(pattern)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, expression)
	}
	return func(path string) bool {
		slashed := filepath.ToSlash(path)
		for _, expression := range compiled {
			if expression.MatchString(slashed) {
				return true
			}
		}
		return false
	}, nil
}

func globToRegexp(glob string) (*regexp.Regexp, error) {
	var pattern strings.Builder
	pattern.WriteString("^")
	for index := 0; index < len(glob); index++ {
		switch {
		case glob[index] == '*' && index+1 < len(glob) && glob[index+1] == '*':
			index++
			if index+1 < len(glob) && glob[index+1] == '/' {
				index++
				pattern.WriteString("(?:.*/)?")
				continue
			}
			pattern.WriteString(".*")
		case glob[index] == '*':
			pattern.WriteString("[^/]*")
		case glob[index] == '?':
			pattern.WriteString("[^/]")
		default:
			pattern.WriteString(regexp.QuoteMeta(string(glob[index])))
		}
	}
	pattern.WriteString("$")
	expression, err := regexp.Compile(pattern.String())
	if err != nil {
		return nil, fmt.Errorf("--exclude %q is not a glob this command reads: %w", glob, err)
	}
	return expression, nil
}
