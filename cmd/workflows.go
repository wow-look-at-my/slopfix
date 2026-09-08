package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/gitmod"
	"github.com/wow-look-at-my/slopfix/workflow"
)

var workflowOnly []string

func init() {
	command := &cobra.Command{
		Use:   "workflows <path>...",
		Short: "Report what the workflow rules reject, and exit 1 when anything does",
		Long: "Walks a directory for every workflow and every action manifest under it, and\n" +
			"reads each by the workflow rules. A named file is read whatever its path.\n\n" +
			"A run that selects no file is a failure, not a pass. The check that scanned\n" +
			"nothing enforced nothing, and a green run there says the opposite.\n\n" +
			"The walk skips this repository's registered submodules, which carry their own\n" +
			"CI. That skip is derived from .gitmodules and the index. There is no flag that\n" +
			"widens it: an exemption a caller writes is one a caller sets to everything.",
		Args: cobra.MinimumNArgs(1),
		RunE: runWorkflows,
	}
	command.Flags().StringSliceVar(&workflowOnly, "only", nil,
		"report only these rule IDs, as a comma-separated list. Defaults to every rule: "+
			slopfix.Listed(workflow.AllIDs))
	rootCmd.AddCommand(command)
}

func runWorkflows(cmd *cobra.Command, args []string) error {
	return reportWorkflows(cmd, args, workflowOnly)
}

// reportWorkflows takes its selection as an argument rather than reading the
// flag global, which parallel tests swap under each other.
func reportWorkflows(cmd *cobra.Command, args, only []string) error {
	reports, err := selectedIDs(only)
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
	submodules, err := gitmod.Skip(arg)
	if err != nil {
		return nil, err
	}
	var out []string
	err = filepath.WalkDir(arg, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if absolute, err := filepath.Abs(path); err == nil && submodules.Contains(absolute) {
				return filepath.SkipDir
			}
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
		if !workflow.AllIDs.Contains(name) {
			return nil, fmt.Errorf("unknown rule %q: pick from %s",
				name, slopfix.Listed(workflow.AllIDs))
		}
		wanted.Add(name)
	}
	return func(id string) bool { return wanted.Contains(id) }, nil
}
