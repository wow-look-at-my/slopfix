// regex.go runs go-regex-compiler. The generated matcher is a switch-based
// automaton, so the shipped binary carries no regexp engine and a run over
// prose with no match allocates nothing.
package gen

import (
	"fmt"
	"os"
	"os/exec"
)

// compilerPackage is the CLI that writes the matchers, run through go run.
const compilerPackage = "github.com/wow-look-at-my/go-regex-compiler/cmd/go-regex-compiler"

// regexJob is a single matcher to write.
type regexJob struct {
	Regex string
	Mode  string
	Func  string
	// Submatch names the capture family, and empty asks for the bool matcher.
	Submatch string
	Names    string
	Out      string
}

// regexCompiler writes a single matcher, and reports what the compiler said
// when it declines the pattern.
func regexCompiler(req Request, job regexJob) error {
	args := []string{"run", compilerPackage,
		"--regex", job.Regex,
		"--match", job.Mode,
		"--package", req.Package,
		"--func", job.Func,
		"--output", job.Out,
	}
	if job.Submatch != "" {
		args = append(args, "--submatch",
			"--submatch-func", job.Submatch,
			"--submatch-names-func", job.Names)
	}
	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go-regex-compiler declined %s: %w", job.Regex, err)
	}
	return nil
}
