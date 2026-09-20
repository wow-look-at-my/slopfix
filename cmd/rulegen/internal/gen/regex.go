// regex.go runs go-regex-compiler. The generated matcher is a switch-based
// automaton, so the shipped binary carries no regexp engine and a run over
// prose with no match allocates nothing.
package gen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// compilerPackage is the CLI that writes the matchers.
const compilerPackage = "github.com/wow-look-at-my/go-regex-compiler/cmd/go-regex-compiler"

var (
	compilerOnce sync.Once
	compilerPath string
	compilerDir  string
	compilerErr  error
)

// compiler builds the CLI a single time and answers the binary.
//
// go run relinks its target on every call, which a table of patterns pays for
// per pattern. Building it a single time turns the generate step from minutes
// into seconds, and every consumer of this module pays that step too.
func compiler() (string, error) {
	compilerOnce.Do(func() {
		compilerDir, compilerErr = os.MkdirTemp("", "rulegen")
		if compilerErr != nil {
			return
		}
		compilerPath = filepath.Join(compilerDir, "go-regex-compiler")
		build := exec.Command("go", "build", "-o", compilerPath, compilerPackage)
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			compilerErr = fmt.Errorf("building %s: %w", compilerPackage, err)
		}
	})
	return compilerPath, compilerErr
}

// DropCompiler removes the binary compiler built.
func DropCompiler() {
	if compilerDir != "" {
		os.RemoveAll(compilerDir)
	}
}

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
	bin, err := compiler()
	if err != nil {
		return err
	}
	args := []string{
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
	cmd := exec.Command(bin, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go-regex-compiler declined %s: %w", job.Regex, err)
	}
	return nil
}
