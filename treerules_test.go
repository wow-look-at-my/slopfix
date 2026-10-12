package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

// gitRepo builds a real repository and stages every file, because the binary
// rule reads what git tracks and the copy rule reads gitattributes.
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	return root
}

// checkOnly runs the repository rule a test is about.
func checkOnly(root, id string) []slopfix.TreeFinding {
	return slopfix.CheckTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleRepo}, IDs: []string{id}}).Findings
}

func pathsOf(findings []slopfix.TreeFinding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, filepath.ToSlash(f.Path)+" "+f.ID)
	}
	return out
}

const elf = "\x7fELF\x02\x01\x01\x00rest of the binary"

func TestATrackedExecutableIsReportedAndFixDeletesIt(t *testing.T) {
	root := gitRepo(t, map[string]string{"bin/tool": elf, "main.go": "package main\n", "app.exe": "MZ\x90\x00"})
	assert.ElementsMatch(t, []string{"app.exe repo/binary", "bin/tool repo/binary"}, pathsOf(checkOnly(root, slopfix.IDBinary)))

	slopfix.FixTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleRepo}, IDs: []string{slopfix.IDBinary}})
	assert.NoFileExists(t, filepath.Join(root, "bin", "tool"))
	assert.NoFileExists(t, filepath.Join(root, "app.exe"))
	assert.FileExists(t, filepath.Join(root, "main.go"))
	assert.Empty(t, checkOnly(root, slopfix.IDBinary))
}

// Git LFS stores a pointer in the index and writes the binary into the
// checkout. Git itself holds no executable, so fix must keep the file.
func TestAGitLFSExecutableIsKept(t *testing.T) {
	pointer := "version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 31\n"
	root := gitRepo(t, map[string]string{
		".gitattributes":      "bin/tool filter=lfs diff=lfs merge=lfs -text\n",
		"bin/tool":            pointer,
		"testdata/fixture.so": elf,
	})
	require.NoError(t, os.WriteFile(filepath.Join(root, "bin", "tool"), []byte(elf), 0o755))
	assert.Equal(t, []string{"testdata/fixture.so repo/binary"}, pathsOf(checkOnly(root, slopfix.IDBinary)))

	slopfix.FixTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleRepo}, IDs: []string{slopfix.IDBinary}})
	assert.FileExists(t, filepath.Join(root, "bin", "tool"))
	assert.NoFileExists(t, filepath.Join(root, "testdata", "fixture.so"))
}

func TestAnUntrackedExecutableIsNotTheRepositorys(t *testing.T) {
	root := gitRepo(t, map[string]string{".gitignore": "build/\n"})
	require.NoError(t, os.MkdirAll(filepath.Join(root, "build"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "build", "tool"), []byte(elf), 0o755))
	assert.Empty(t, checkOnly(root, slopfix.IDBinary))
}

func TestATreeGitCannotListIsReadFromDisk(t *testing.T) {
	root := gitRoot(t, map[string]string{"tool": elf})
	assert.Equal(t, []string{"tool repo/binary"}, pathsOf(checkOnly(root, slopfix.IDBinary)))
}

// script is long enough that a single added comment line keeps it over the share.
var script = func() string {
	lines := []string{"import { run } from './run';"}
	for _, step := range []string{"read", "parse", "check", "build", "write", "send", "log", "close", "sync", "wait"} {
		lines = append(lines, "export function "+step+"(n: number) {", "  return run('"+step+"', n);", "}")
	}
	return strings.Join(lines, "\n") + "\n"
}()

func TestTwoFilesOfOneNameThatMatchAreReported(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"a/run.ts":   script,
		"b/run.ts":   "// The copy differs in a comment alone.\n" + script,
		"c/run.ts":   "export const other = true;\n",
		"d/other.ts": script,
	})
	findings := checkOnly(root, slopfix.IDNearDuplicate)
	require.Len(t, findings, 1)
	assert.Equal(t, "b/run.ts", filepath.ToSlash(findings[0].Path))
	assert.Contains(t, findings[0].Rule, "a/run.ts")
}

// Each copy is reported once, so the report grows with the copies and not with the pairs.
func TestEachCopyIsReportedOnce(t *testing.T) {
	root := gitRepo(t, map[string]string{"a/run.ts": script, "b/run.ts": script, "c/run.ts": script})
	assert.Equal(t, []string{"b/run.ts repo/near-duplicate", "c/run.ts repo/near-duplicate"}, pathsOf(checkOnly(root, slopfix.IDNearDuplicate)))
}

// No attribute exempts a copy. The only answer to a copy is one file.
func TestNoAttributeExemptsACopy(t *testing.T) {
	root := gitRepo(t, map[string]string{
		".gitattributes":  "copies/** slopfix-copy\n",
		"copies/a/run.ts": script,
		"copies/b/run.ts": script,
	})
	assert.Equal(t, []string{"copies/b/run.ts repo/near-duplicate"}, pathsOf(checkOnly(root, slopfix.IDNearDuplicate)))
}

// A symlink is the file it names, so it is the answer to a copy and never a copy itself.
func TestASymlinkIsNotACopy(t *testing.T) {
	root := gitRepo(t, map[string]string{"a/run.ts": script, "b/.keep": ""})
	require.NoError(t, os.Symlink("../a/run.ts", filepath.Join(root, "b", "run.ts")))
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Empty(t, checkOnly(root, slopfix.IDNearDuplicate))
}

func TestFilesEachDirectoryNeedsAreNotCopies(t *testing.T) {
	manifest := "{\n  \"name\": \"x\",\n  \"private\": true\n}\n"
	root := gitRepo(t, map[string]string{"a/package.json": manifest, "b/package.json": manifest, "a/.keep": "", "b/.keep": ""})
	assert.Empty(t, checkOnly(root, slopfix.IDNearDuplicate))
}

// ts0, pnpm and each read the file in the directory they build. The
// repository-scripts repair writes a justfile beside each manifest.
func TestBuildToolFilesInEachActionAreNotCopies(t *testing.T) {
	ts0 := "{\n\t\"entry\": \"src/index.ts\",\n\t\"outfile\": \"dist/index.js\",\n\t\"target\": \"node\",\n\t\"format\": \"cjs\"\n}\n"
	workspace := "allowBuilds:\n  esbuild: true\n"
	recipes := "[private]\nhelp:\n\t@just --list\n\nbuild:\n\tpnpm install\n\tts0 build\n"
	files := map[string]string{}
	for _, dir := range []string{"cache-cleanup", "cache-upload"} {
		files[dir+"/ts0.json"] = ts0
		files[dir+"/pnpm-workspace.yaml"] = workspace
		files[dir+"/justfile"] = recipes
	}
	assert.Empty(t, checkOnly(gitRepo(t, files), slopfix.IDNearDuplicate))
}
