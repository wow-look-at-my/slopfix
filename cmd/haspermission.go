package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/workflow"
)

type permissionRequest struct {
	workflow, job, permission, level string
	assert                           bool
}

var permissionFlags permissionRequest

func init() {
	command := &cobra.Command{
		Use:   "has-permission --permission NAME",
		Short: "Report whether a workflow job holds a permission",
		Long: "Report the level a job's permissions block, or else its workflow's, gives one permission. " +
			"In a GitHub Actions step the workflow file and the job default to the running ones. " +
			"The answer is one JSON object on stdout.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return hasPermission(cmd, permissionFlags, os.Getenv) },
	}
	f := command.Flags()
	f.StringVar(&permissionFlags.permission, "permission", "", "the permission scope, such as id-token, contents or packages")
	f.StringVar(&permissionFlags.level, "level", "write", "the level the caller needs: none, read or write. write covers read")
	f.StringVar(&permissionFlags.workflow, "workflow", "", "the workflow file. Defaults to the one GITHUB_WORKFLOW_REF names")
	f.StringVar(&permissionFlags.job, "job", "", "the job key. Defaults to GITHUB_JOB")
	f.BoolVar(&permissionFlags.assert, "assert", false, "exit 1 when the job does not hold the level")
	rootCmd.AddCommand(command)
}

// permissionAnswer is this command's wire contract.
type permissionAnswer struct {
	Granted bool   `json:"granted"`
	Level   string `json:"level"`
	Source  string `json:"source"`
	// Message says why, in words a log reader acts on.
	Message string `json:"message"`
}

// hasPermission takes its environment as a function, so a test never sets the process environment.
func hasPermission(cmd *cobra.Command, req permissionRequest, getenv func(string) string) error {
	if req.permission == "" {
		return errors.New("--permission is required")
	}
	if !slices.Contains(workflow.Levels, req.level) {
		return fmt.Errorf("--level is %q, not none, read or write", req.level)
	}
	file, err := workflowFile(req.workflow, getenv)
	if err != nil {
		return err
	}
	job := req.job
	if job == "" {
		job = getenv("GITHUB_JOB")
	}
	if job == "" {
		return errors.New("--job is required outside a GitHub Actions step")
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("%w. Check the repository out before this step", err)
	}
	grant, err := workflow.Permission(string(content), job, req.permission)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}

	answer := permissionAnswer{Granted: grant.Covers(req.level), Level: grant.Level, Source: grant.Source}
	where := fmt.Sprintf("%s job '%s'", file, job)
	switch {
	case answer.Granted:
		answer.Message = fmt.Sprintf("%s: %s is granted to %s by its %s permissions block", req.permission, grant.Level, where, grant.Source)
	case grant.Source == "default":
		answer.Message = fmt.Sprintf("%s declares no permissions block, so %s falls to the repository default. Declare the block to grant it.", where, req.permission)
	default:
		answer.Message = fmt.Sprintf("%s: %s is NOT granted to %s. Its %s permissions block gives %s.", req.permission, req.level, where, grant.Source, grant.Level)
	}
	if err := json.NewEncoder(cmd.OutOrStdout()).Encode(answer); err != nil {
		return err
	}
	if req.assert && !answer.Granted {
		return errors.New(answer.Message)
	}
	return nil
}

// workflowFile answers the named file, or the running workflow's. GITHUB_WORKFLOW_REF
// reads owner/repo/.github/workflows/ci.yml@refs/heads/main.
func workflowFile(named string, getenv func(string) string) (string, error) {
	if named != "" {
		return named, nil
	}
	ref := getenv("GITHUB_WORKFLOW_REF")
	if ref == "" {
		return "", errors.New("--workflow is required outside a GitHub Actions step")
	}
	path, _, _ := strings.Cut(ref, "@")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return "", fmt.Errorf("GITHUB_WORKFLOW_REF is %q, which names no file", ref)
	}
	return filepath.Join(getenv("GITHUB_WORKSPACE"), parts[2]), nil
}
