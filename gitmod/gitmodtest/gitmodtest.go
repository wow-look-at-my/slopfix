// Package gitmodtest builds the repositories the submodule skip is judged
// against.
//
// They are real repositories, made with git, because the rule reads the index.
// A fake .gitmodules alone proves nothing: telling a real submodule from a
// forged one IS the rule, and only git can put a gitlink in an index.
package gitmodtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// RepoWithSubmodule returns a work tree carrying a real submodule at path.
func RepoWithSubmodule(t *testing.T, path string) string {
	t.Helper()
	root := newRepo(t)

	inner := newRepo(t)
	write(t, inner, "README.md", "vendored\n")
	run(t, inner, "add", "-A")
	run(t, inner, "commit", "-m", "seed")

	run(t, root, "-c", "protocol.file.allow=always", "submodule", "add", inner, path)
	run(t, root, "commit", "-m", "add "+path)
	return root
}

// RepoWithFakeSubmodule returns a work tree whose .gitmodules claims path is a
// submodule while path holds ordinary tracked files. This is the forgery the
// rule has to reject: were a declaration enough, any repository could name its
// own source and stop being read.
func RepoWithFakeSubmodule(t *testing.T, path string) string {
	t.Helper()
	root := newRepo(t)
	write(t, root, filepath.Join(path, "keep.txt"), "real source\n")
	write(t, root, ".gitmodules", "[submodule \""+path+"\"]\n\tpath = "+path+"\n\turl = ./nowhere\n")
	run(t, root, "add", "-A")
	run(t, root, "commit", "-m", "seed")
	return root
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "config", "user.email", "test@example.invalid")
	run(t, dir, "config", "user.name", "test")
	write(t, dir, ".keep", "")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-m", "init")
	return dir
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	out, err := command.CombinedOutput()
	require.NoError(t, err, "git %v in %s: %s", args, dir, out)
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	full := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(body), 0o644))
}
