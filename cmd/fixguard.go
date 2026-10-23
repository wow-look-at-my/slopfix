// fixWorkTree refuses a repair that writes files outside one git work tree.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/slopfix/gitmod"
)

// fixWorkTree refuses a repair that writes files outside one git work tree.
// The working directory must lie in a work tree. Every path must lie in that
// same tree, not in a nested repository or a submodule of it. A run from a
// directory that holds many repositories would otherwise rewrite them all.
func fixWorkTree(paths []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, ok := gitmod.TopLevel(cwd)
	if !ok {
		return fmt.Errorf("slopfix fix writes files, so it runs only inside a git work tree, and %s is in none", cwd)
	}
	for _, path := range paths {
		abs, err := gitmod.Resolve(path)
		if err != nil {
			return err
		}
		if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
			return fmt.Errorf("slopfix fix writes only inside the work tree %s, and %s is outside it", root, path)
		}
		own, ok := gitmod.TopLevel(abs)
		if !ok || own != root {
			return fmt.Errorf("slopfix fix writes only inside the work tree %s, and %s belongs to the work tree %s", root, path, own)
		}
	}
	return nil
}
