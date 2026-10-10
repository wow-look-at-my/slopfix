// Package ratchet holds a branch to the default branch's guarantee. The
// default branch's check judges what the branch's fix writes, over every case
// the default branch registers. A branch that drops a rule, loosens a cap,
// narrows a detection or stops repairing leaves text that check still flags.
package ratchet

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/slopfix"
)

// Judge pairs the binary that judges with the binary under test.
type Judge struct {
	// Base is the default branch's slopfix. Its check is the verdict.
	Base string
	// Head is the branch's slopfix. Its fix writes what Base judges.
	Head string
	// Work is a directory each case is written under.
	Work string
}

// Failure is a case Base still flags after Head repaired it.
type Failure struct {
	Rule string
	Case string
	// Repair is what Head printed while it repaired the case.
	Repair string
	// Verdict is what Base printed while it judged the result.
	Verdict string
}

func (f Failure) String() string {
	return fmt.Sprintf("%s, case %q:\n%s%s", f.Rule, f.Case, f.Verdict, f.Repair)
}

// Run repairs each case with Head and judges it with Base. It answers every
// case Base still flags.
func (j Judge) Run(specs []slopfix.RuleSpec) ([]Failure, error) {
	var failures []Failure
	for _, spec := range specs {
		for i, c := range spec.Cases {
			dir := filepath.Join(j.Work, strings.ReplaceAll(spec.ID, "/", "_"), strconv.Itoa(i))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
			tree := c.Path != "" || len(c.Files) > 0
			materialized, err := slopfix.Materialize(dir, c)
			if err != nil {
				return nil, fmt.Errorf("%s, case %q: %w", spec.ID, c.Name, err)
			}
			if tree {
				repair, _ := run(j.Head, dir, "", "fix", ".")
				if verdict, err := run(j.Base, dir, "", "check", "."); err != nil {
					failures = append(failures, Failure{Rule: spec.ID, Case: c.Name, Repair: repair, Verdict: verdict})
				}
			}
			if materialized.Text == "" {
				continue
			}
			repaired, err := message(j.Head, dir, materialized.Text)
			if err != nil {
				failures = append(failures, Failure{Rule: spec.ID, Case: c.Name, Repair: err.Error()})
				continue
			}
			if verdict, err := run(j.Base, dir, repaired, "check", "--message"); err != nil {
				failures = append(failures, Failure{Rule: spec.ID, Case: c.Name, Verdict: verdict})
			}
		}
	}
	return failures, nil
}

// message repairs a closing message with bin, and answers the repaired text.
func message(bin, dir, text string) (string, error) {
	cmd := exec.Command(bin, "check", "--message", "--fix")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(text)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if _, exited := err.(*exec.ExitError); !exited {
			return "", fmt.Errorf("%s check --message --fix: %w", bin, err)
		}
	}
	return stdout.String(), nil
}

// run runs bin in dir with stdin, and answers its combined output. A non-zero
// exit is the error.
func run(bin, dir, stdin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Build compiles the slopfix command of the module at dir into out.
func Build(dir, out string) error {
	cmd := exec.Command("go", "build", "-o", out, "./cmd/slopfix")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build in %s: %w: %s", dir, err, output)
	}
	return nil
}
