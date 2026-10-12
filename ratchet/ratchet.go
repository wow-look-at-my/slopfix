// Package ratchet holds a branch to the default branch's guarantee. The
// default branch's check judges what the branch's fix writes, over every case
// the default branch registers. A branch that drops a rule, loosens a cap,
// narrows a detection or stops repairing leaves text that check still flags.
package ratchet

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/forkscope"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// Judge pairs the binary that judges with the binary under test.
type Judge struct {
	// Base is the default branch's slopfix. Its check is the verdict.
	Base string
	// Head is the branch's slopfix. Its fix writes what Base judges.
	Head string
	// Work is a directory each case is written under.
	Work string
	// Repair and Check decide a case. Nil runs Base and Head as programs.
	Repair Decider
	Check  Decider
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

// Decider repairs a case, or judges it. The shipped one runs bin as a program.
type Decider func(bin, dir, stdin string, args ...string) (string, error)

// Repair answers how a case is written: the shipped decider runs Head.
func (j Judge) repair(bin, dir, stdin string, args ...string) (string, error) {
	if j.Repair != nil {
		return j.Repair(bin, dir, stdin, args...)
	}
	return run(bin, dir, stdin, args...)
}

// Check answers how a case is judged, which is what decides a failure: the
// shipped decider runs Base.
func (j Judge) check(bin, dir, stdin string, args ...string) (string, error) {
	if j.Check != nil {
		return j.Check(bin, dir, stdin, args...)
	}
	return run(bin, dir, stdin, args...)
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
				repair, _ := j.repair(j.Head, dir, "", "fix", ".")
				if verdict, err := j.check(j.Base, dir, "", "check", "."); err != nil {
					failures = append(failures, Failure{Rule: spec.ID, Case: c.Name, Repair: repair, Verdict: verdict})
				}
			}
			if materialized.Text == "" {
				continue
			}
			repaired, err := j.message(dir, materialized.Text)
			if err != nil {
				failures = append(failures, Failure{Rule: spec.ID, Case: c.Name, Repair: err.Error()})
				continue
			}
			if verdict, err := j.check(j.Base, dir, repaired, "check", "--message"); err != nil {
				failures = append(failures, Failure{Rule: spec.ID, Case: c.Name, Verdict: verdict})
			}
		}
	}
	return failures, nil
}

// message repairs a closing message with Head, and answers the repaired text.
func (j Judge) message(dir, text string) (string, error) {
	return j.repair(j.Head, dir, text, "check", "--message", "--fix")
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

// InProcess answers a decider that reaches slopfix's entry points in process.
func InProcess() Decider {
	return func(_, dir, stdin string, args ...string) (string, error) {
		return inProcess(dir, stdin, args)
	}
}

// errFlagged is the verdict a check answers when it flags something.
var errFlagged = fmt.Errorf("flagged")

// inProcess answers one slopfix command, and errFlagged when a check flags.
func inProcess(dir, stdin string, args []string) (out string, err error) {
	back, err := os.Getwd()
	if err != nil {
		return "", err
	}
	defer os.Chdir(back)
	if err := os.Chdir(dir); err != nil {
		return "", err
	}
	message, repairing := false, false
	for _, a := range args {
		switch a {
		case "--message":
			message = true
		case "--fix", "fix":
			repairing = true
		}
	}
	if message {
		return messageRun(stdin, repairing)
	}
	// The shipped run judges the working directory with every rule.
	request := slopfix.Request{MaxCommentLines: tombstones.DefaultMaxCommentLines}
	own, err := forkscope.Resolver{}.Lines(".")
	if err != nil {
		return "", err
	}
	request.Fork = own
	if repairing {
		return "", flagged(slopfix.FixTreeWith(".", request).Within(own, "."))
	}
	return "", flagged(slopfix.CheckTreeWith(".", request).Within(own, "."))
}

// messageRun answers the repaired text, which is what the judge feeds Base.
func messageRun(text string, repairing bool) (string, error) {
	runs := func(string) bool { return true }
	if repairing {
		return slopfix.FixMessage(text, runs), nil
	}
	findings := slopfix.CheckMessage(text, runs)
	if len(findings) > 0 {
		return "", errFlagged
	}
	return "", nil
}

// flagged answers errFlagged for a finding a check reports, a tombstone no
// deletion resolves, or a fixture expectation that did not hold.
func flagged(tree slopfix.TreeRepair) error {
	for _, f := range tree.Findings {
		if !f.Finding.Warning() {
			return errFlagged
		}
	}
	if len(tree.Kept) > 0 || len(tree.Unmet) > 0 {
		return errFlagged
	}
	return nil
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
