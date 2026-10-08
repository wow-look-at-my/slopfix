// Command ratchet runs from the default branch's checkout and judges the branch
// checked out at its one argument. go-toolchain runs it, as .github/ratchet
// names it, on every branch's CI run.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ratchet"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: ratchet BRANCH_CHECKOUT")
	}
	work, err := os.MkdirTemp("", "ratchet")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	judge := ratchet.Judge{
		Base: filepath.Join(work, "base"),
		Head: filepath.Join(work, "head"),
		Work: filepath.Join(work, "cases"),
	}
	if err := ratchet.Build(args[0], judge.Head); err != nil {
		return err
	}
	if err := ratchet.Build(".", judge.Base); err != nil {
		return err
	}
	failures, err := judge.Run(slopfix.AllRuleSpecs())
	if err != nil {
		return err
	}
	for _, f := range failures {
		fmt.Fprintln(os.Stderr, f)
	}
	if len(failures) > 0 {
		return fmt.Errorf("ratchet: the default branch's check still flags %d case(s) after this branch's fix. "+
			"The branch weakened the guarantee: a rule it dropped, a cap it loosened, a detection it narrowed, or a repair it stopped making. "+
			"Fix the repair", len(failures))
	}
	return nil
}
