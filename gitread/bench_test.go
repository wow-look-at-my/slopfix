package gitread_test

import (
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/gitread"
)

// Each pair times one question: in-process, then as the git process that
// answered it before. The fixture is a tree of many files with history.

func benchRepo(b *testing.B) string {
	b.Helper()
	dir := newRepo(b)
	for i := 0; i < 200; i++ {
		writeT(b, dir, "pkg"+strconv.Itoa(i%10)+"/file"+strconv.Itoa(i)+".go", "package p\n\nvar X"+strconv.Itoa(i)+" = 1\n")
	}
	gitT(b, dir, "add", "-A")
	gitT(b, dir, "commit", "-q", "-m", "files")
	gitT(b, dir, "gc", "-q")
	return dir
}

func runGit(b *testing.B, dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		exit, ok := err.(*exec.ExitError)
		require.False(b, !ok || exit.ExitCode() > 1)

	}
}

func BenchmarkAttr(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		repo, _ := gitread.OpenWorkTree(dir)
		_ = repo.Attr("pkg3/file13.go", "linguist-vendored")
	}
}

func BenchmarkAttrGit(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		runGit(b, dir, "check-attr", "-z", "linguist-vendored", "--", "pkg3/file13.go")
	}
}

func BenchmarkIgnore(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		repo, _ := gitread.OpenWorkTree(dir)
		_ = repo.Ignore().Ignored("sub/x/anchored.txt", false)
	}
}

func BenchmarkIgnoreGit(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		runGit(b, dir, "check-ignore", "-q", "sub/x/anchored.txt")
	}
}

func BenchmarkIndex(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		repo, _ := gitread.OpenWorkTree(dir)
		_, err := repo.Index()
		require.Nil(b, err)

	}
}

func BenchmarkIndexGit(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		runGit(b, dir, "ls-files", "-s", "-z")
	}
}

func BenchmarkWorkFiles(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		repo, _ := gitread.OpenWorkTree(dir)
		_, err := repo.WorkFiles()
		require.Nil(b, err)

	}
}

func BenchmarkWorkFilesGit(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		runGit(b, dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	}
}

func BenchmarkBlobAt(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		repo, _ := gitread.OpenWorkTree(dir)
		head, _ := repo.Head()
		_, err := repo.BlobAt(head, "pkg3/file13.go")
		require.Nil(b, err)

	}
}

func BenchmarkBlobAtGit(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		runGit(b, dir, "cat-file", "blob", "HEAD:pkg3/file13.go")
	}
}

func BenchmarkMergeBase(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		repo, _ := gitread.OpenWorkTree(dir)
		head, _ := repo.Head()
		feature, _ := repo.Resolve("feature")
		_, _, err := repo.MergeBase(head, feature)
		require.Nil(b, err)

	}
}

func BenchmarkMergeBaseGit(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		runGit(b, dir, "merge-base", "HEAD", "feature")
	}
}

func BenchmarkChangedNames(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		repo, _ := gitread.OpenWorkTree(dir)
		base, _ := repo.Resolve("v1")
		_, err := repo.ChangedNames(base)
		require.Nil(b, err)

	}
}

func BenchmarkChangedNamesGit(b *testing.B) {
	dir := benchRepo(b)
	for b.Loop() {
		runGit(b, dir, "diff", "--name-only", "--no-renames", "v1", "--")
		runGit(b, dir, "ls-files", "--others", "--exclude-standard")
	}
}
