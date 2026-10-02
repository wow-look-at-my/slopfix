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

// permissionRequest is check --permission: does a workflow job hold a permission.
type permissionRequest struct {
	workflow, job, permission, level string
	asJSON                           bool
}

// permissionAnswer is the --json wire contract.
type permissionAnswer struct {
	Granted bool   `json:"granted"`
	Level   string `json:"level"`
	Source  string `json:"source"`
	// Message says why, in words a log reader acts on.
	Message string `json:"message"`
}

// checkPermission takes its environment as a function, so a test never sets the process environment.
func checkPermission(cmd *cobra.Command, req permissionRequest, getenv func(string) string) error {
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
	if req.asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(answer)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), answer.Message)
	if !answer.Granted {
		return errFindings
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
		return "", errors.New("name the workflow file outside a GitHub Actions step")
	}
	path, _, _ := strings.Cut(ref, "@")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return "", fmt.Errorf("GITHUB_WORKFLOW_REF is %q, which names no file", ref)
	}
	return filepath.Join(getenv("GITHUB_WORKSPACE"), parts[2]), nil
}
