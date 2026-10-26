package gitread_test

import _ "embed"

import (
	"bytes"
	"crypto/sha1"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitread"
)

func gitT(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "protocol.file.allow=always"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

// gitZ runs git and splits its NUL-terminated output.
func gitZ(t testing.TB, dir string, args ...string) []string {
	t.Helper()
	var out []string
	for _, field := range strings.Split(gitT(t, dir, args...), "\x00") {
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func writeT(t testing.TB, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

// newRepo builds a repository with history, a tag, a merge, attributes and ignore rules.
func newRepo(t testing.TB) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "-b", "main")
	writeT(t, dir, "doc.md", "# Doc\n\nAlpha.\n")
	writeT(t, dir, "sub/deep.go", "package p\n\nvar A = 1\n")
	writeT(t, dir, ".gitattributes", "isa/** linguist-vendored\nroutes.txt linguist-generated\n[attr]borrowed linguist-vendored\nthird/** borrowed\n*.bin binary\n")
	writeT(t, dir, ".gitignore", "out.go\n/build/\n*.log\n!keep.log\ndocs/**/tmp\n[Tt]emp*\n")
	writeT(t, dir, "sub/.gitignore", "local.txt\n/anchored.txt\n")
	writeT(t, dir, "sub/.gitattributes", "*.go -linguist-vendored\nnested.txt linguist-generated=true\n")
	writeT(t, dir, "isa/ch01.go", "package p\n")
	writeT(t, dir, "third/lib.c", "int x;\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "base")
	gitT(t, dir, "tag", "-a", "-m", "v1", "v1")

	gitT(t, dir, "checkout", "-q", "-b", "feature")
	writeT(t, dir, "doc.md", newRepoMd)
	writeT(t, dir, "sub/deep.go", "package p\n\nvar A = 2\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "feature")

	gitT(t, dir, "checkout", "-q", "main")
	writeT(t, dir, "other.md", "Other.\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "main moves")
	gitT(t, dir, "merge", "-q", "--no-ff", "-m", "merge feature", "feature")

	for _, name := range []string{"out.go", "build/a.o", "x.log", "keep.log", "docs/a/b/tmp", "Temp1", "temp2", "sub/local.txt", "sub/anchored.txt", "sub/x/anchored.txt", "fresh.md", "sub/fresh.go"} {
		writeT(t, dir, name, "untracked\n")
	}
	return dir
}

func open(t testing.TB, dir string) *gitread.Repo {
	t.Helper()
	repo, err := gitread.OpenWorkTree(dir)
	require.NoError(t, err)
	require.NotNil(t, repo, "the work tree is not a repository")
	return repo
}

func TestHeadTagsAndRevList(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)

	head, err := repo.Head()
	require.NoError(t, err)
	assert.Equal(t, gitT(t, dir, "rev-parse", "HEAD"), head.String())

	tag, err := repo.Resolve("refs/tags/v1")
	require.NoError(t, err)
	peeled, err := repo.PeelCommit(tag)
	require.NoError(t, err)
	assert.Equal(t, gitT(t, dir, "rev-parse", "v1^{commit}"), peeled.String())

	commits, err := repo.RevList(head)
	require.NoError(t, err)
	var got []string
	for _, c := range commits {
		got = append(got, c.String())
	}
	assert.Equal(t, strings.Fields(gitT(t, dir, "rev-list", "HEAD")), got, "the walk keeps git's order")
}

func TestResolveSpellsRevisionsAsGitDoes(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)
	short := gitT(t, dir, "rev-parse", "--short=7", "HEAD~1")
	for _, rev := range []string{"HEAD", "@", "HEAD~1", "HEAD^2", "HEAD~2", "main", "feature", "v1", "v1^{commit}", "tags/v1", "heads/feature", "HEAD^{tree}", short, "main^1~1"} {
		oid, err := repo.Resolve(rev)
		require.NoError(t, err, rev)
		assert.Equal(t, gitT(t, dir, "rev-parse", rev), oid.String(), rev)
	}
	_, err := repo.Resolve("HEAD@{1}")
	assert.ErrorIs(t, err, gitread.ErrUnknownRevision)
}

func TestMergeBaseAndAncestry(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)
	gitT(t, dir, "checkout", "-q", "-b", "side", "v1")
	writeT(t, dir, "side.md", "Side.\n")
	gitT(t, dir, "add", "side.md")
	gitT(t, dir, "commit", "-q", "-m", "side")
	gitT(t, dir, "checkout", "-q", "main")
	repo = open(t, dir)

	for _, pair := range [][2]string{{"main", "feature"}, {"main", "side"}, {"feature", "side"}, {"main", "main"}, {"v1", "main"}} {
		a, err := repo.Resolve(pair[0])
		require.NoError(t, err)
		b, err := repo.Resolve(pair[1])
		require.NoError(t, err)
		base, ok, err := repo.MergeBase(a, b)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, gitT(t, dir, "merge-base", pair[0], pair[1]), base.String(), pair)

		want := exec.Command("git", "merge-base", "--is-ancestor", pair[0], pair[1])
		want.Dir = dir
		is, err := repo.IsAncestor(a, b)
		require.NoError(t, err)
		assert.Equal(t, want.Run() == nil, is, "is %s an ancestor of %s", pair[0], pair[1])
	}
}

func TestTreesBlobsAndDiffs(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)
	head, err := repo.Head()
	require.NoError(t, err)

	entry, ok, err := repo.TreeEntryAt(head, "sub/deep.go")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "100644", entry.Mode)

	blob, err := repo.BlobAt(head, "sub/deep.go")
	require.NoError(t, err)
	assert.Contains(t, string(blob), "var A = 2")

	_, ok, err = repo.TreeEntryAt(head, "gone.go")
	require.NoError(t, err)
	assert.False(t, ok)

	tagged, err := repo.Resolve("v1^{commit}")
	require.NoError(t, err)
	names, err := repo.ChangedPaths(tagged, head)
	require.NoError(t, err)
	assert.Equal(t, strings.Fields(gitT(t, dir, "diff", "--name-only", "--no-renames", "v1", "HEAD")), names)

	all, err := repo.TreeNames(head)
	require.NoError(t, err)
	var paths []string
	for path := range all {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	assert.Equal(t, gitZ(t, dir, "ls-tree", "-r", "-z", "--name-only", "HEAD"), paths)
}

func TestIndexEntries(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)
	idx, err := repo.Index()
	require.NoError(t, err)
	var got []string
	for _, e := range idx.Entries() {
		got = append(got, e.Mode+" "+e.OID.String()+" "+strconv.Itoa(e.Stage)+"\t"+e.Path)
	}
	assert.Equal(t, gitZ(t, dir, "ls-files", "-s", "-z"), got)

	// A new write to the index is read again.
	writeT(t, dir, "late.md", "Late.\n")
	gitT(t, dir, "add", "late.md")
	idx, err = repo.Index()
	require.NoError(t, err)
	assert.Contains(t, idx.Tracked(), "late.md")
}

func TestAttributesMatchCheckAttr(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)
	writeT(t, dir, ".git/info/attributes", "info.md linguist-generated\n")
	paths := []string{"isa/ch01.go", "routes.txt", "doc.md", "third/lib.c", "sub/deep.go", "sub/nested.txt", "info.md", "a.bin", "sub/x/routes.txt", "isa/deep/x.txt"}
	attrs := repo.Attributes()
	for _, name := range []string{"linguist-vendored", "linguist-generated", "diff", "binary"} {
		out := gitZ(t, dir, append([]string{"check-attr", "-z", name, "--"}, paths...)...)
		require.Len(t, out, len(paths)*3)
		for i := 0; i < len(out); i += 3 {
			assert.Equal(t, out[i+2], attrs.Value(out[i], name), "%s on %s", name, out[i])
		}
	}
}

func TestIgnoreMatchesCheckIgnore(t *testing.T) {
	dir := newRepo(t)
	writeT(t, dir, ".git/info/exclude", "excluded.md\n")
	writeT(t, dir, "excluded.md", "x\n")
	repo := open(t, dir)
	paths := []string{"out.go", "build/a.o", "build", "x.log", "keep.log", "docs/a/b/tmp", "Temp1", "temp2", "sub/local.txt",
		"sub/anchored.txt", "sub/x/anchored.txt", "fresh.md", "sub/fresh.go", "excluded.md", "sub/out.go", "deep/build/x"}
	cmd := exec.Command("git", "check-ignore", "--no-index", "-z", "--stdin")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\x00") + "\x00")
	raw, _ := cmd.Output()
	require.NotEmpty(t, raw, "git ignores some of the paths")
	want := set.New[string]()
	for _, p := range strings.Split(string(raw), "\x00") {
		want.Add(p)
	}
	ignore := repo.Ignore()
	for _, p := range paths {
		info, err := os.Stat(filepath.Join(dir, p))
		isDir := err == nil && info.IsDir()
		assert.Equal(t, want.Contains(p), ignore.Ignored(p, isDir), p)
	}
}

func TestWorkFilesMatchLsFiles(t *testing.T) {
	dir := newRepo(t)
	nested := filepath.Join(dir, "vendored-repo")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	gitT(t, nested, "init", "-q")
	repo := open(t, dir)

	files, err := repo.WorkFiles()
	require.NoError(t, err)
	want := gitZ(t, dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	sort.Strings(want)
	assert.Equal(t, want, files)

	untracked, err := repo.Untracked()
	require.NoError(t, err)
	want = gitZ(t, dir, "ls-files", "-z", "--others", "--exclude-standard")
	sort.Strings(want)
	assert.Equal(t, want, untracked)
}

func TestChangedNamesMatchDiff(t *testing.T) {
	dir := newRepo(t)
	writeT(t, dir, "doc.md", "# Doc\n\nEdited.\n")
	require.NoError(t, os.Remove(filepath.Join(dir, "other.md")))
	gitT(t, dir, "rm", "-q", "--cached", "third/lib.c")
	require.NoError(t, os.Chmod(filepath.Join(dir, "sub/deep.go"), 0o755))
	repo := open(t, dir)
	base, err := repo.Resolve("v1")
	require.NoError(t, err)

	names, err := repo.ChangedNames(base)
	require.NoError(t, err)
	// A path the index dropped is both deleted and untracked.
	want := set.Of(append(gitZ(t, dir, "diff", "-z", "--name-only", "--no-renames", "v1", "--"), gitZ(t, dir, "ls-files", "-z", "--others", "--exclude-standard")...)...).Values()
	sort.Strings(want)
	assert.Equal(t, want, names)
}

func TestTrackedRecursiveEntersSubmodules(t *testing.T) {
	dir := newRepo(t)
	lib := t.TempDir()
	gitT(t, lib, "init", "-q", "-b", "main")
	writeT(t, lib, "inner/x.go", "package x\n")
	gitT(t, lib, "add", "-A")
	gitT(t, lib, "commit", "-q", "-m", "lib")
	gitT(t, dir, "submodule", "add", "-q", lib, "deps/lib")
	repo := open(t, dir)

	got, err := repo.TrackedRecursive()
	require.NoError(t, err)
	assert.ElementsMatch(t, gitZ(t, dir, "ls-files", "-z", "--cached", "--recurse-submodules"), got)

	files, err := repo.WorkFiles()
	require.NoError(t, err)
	want := gitZ(t, dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	sort.Strings(want)
	assert.Equal(t, want, files, "the walk does not enter a submodule")
}

// verifyAllObjects reads every object git lists and checks its name against
// its content, which proves each delta chain resolved.
func verifyAllObjects(t *testing.T, dir string) {
	t.Helper()
	repo := open(t, dir)
	listed := strings.Fields(gitT(t, dir, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)"))
	require.NotEmpty(t, listed)
	for _, hex := range listed {
		oid, err := gitread.ParseOID(hex)
		require.NoError(t, err)
		typ, data, err := repo.Object(oid)
		require.NoError(t, err, hex)
		sum := sha1.Sum(append([]byte(typ+" "+strconv.Itoa(len(data))+"\x00"), data...))
		assert.Equal(t, hex, gitread.OID(sum).String())
		assert.True(t, repo.Has(oid))
	}
}

// historyRepo builds enough similar revisions of a file for git to delta them.
func historyRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "-b", "main")
	var body bytes.Buffer
	for i := 0; i < 40; i++ {
		for j := 0; j < 50; j++ {
			body.WriteString("line " + strconv.Itoa(j) + " of a long and similar file\n")
		}
		body.WriteString("revision " + strconv.Itoa(i) + "\n")
		writeT(t, dir, "big.txt", body.String())
		writeT(t, dir, "dir"+strconv.Itoa(i%4)+"/f.txt", body.String()[:body.Len()/2])
		gitT(t, dir, "add", "-A")
		gitT(t, dir, "commit", "-q", "-m", "rev "+strconv.Itoa(i))
	}
	return dir
}

func TestObjectsReadLooseAndPacked(t *testing.T) {
	dir := historyRepo(t)
	verifyAllObjects(t, dir)

	gitT(t, dir, "repack", "-adf", "-q")
	verifyAllObjects(t, dir)

	gitT(t, dir, "-c", "repack.useDeltaBaseOffset=false", "repack", "-adf", "-q")
	verifyAllObjects(t, dir)
}

func TestARepackAfterOpenIsFound(t *testing.T) {
	dir := historyRepo(t)
	repo := open(t, dir)
	_, err := repo.Head()
	require.NoError(t, err)
	gitT(t, dir, "gc", "-q", "--prune=now")
	writeT(t, dir, "after.txt", "after\n")
	gitT(t, dir, "add", "after.txt")
	gitT(t, dir, "commit", "-q", "-m", "after")
	gitT(t, dir, "repack", "-adq")
	head, err := repo.Head()
	require.NoError(t, err)
	_, err = repo.BlobAt(head, "after.txt")
	assert.NoError(t, err)
}

func TestARepositoryIsSharedAndRevalidated(t *testing.T) {
	dir := newRepo(t)
	a := open(t, dir)
	b := open(t, filepath.Join(dir, "sub"))
	assert.Same(t, a, b, "one repository serves every caller")

	require.NoError(t, os.RemoveAll(filepath.Join(dir, ".git")))
	gitT(t, dir, "init", "-q")
	c := open(t, dir)
	assert.NotSame(t, a, c, "a new repository at the same path opens fresh")
}

func TestShallowBoundaryHasNoParents(t *testing.T) {
	src := historyRepo(t)
	dir := filepath.Join(t.TempDir(), "shallow")
	gitT(t, src, "clone", "-q", "--depth", "3", "file://"+src, dir)
	repo := open(t, dir)
	assert.True(t, repo.IsShallow())
	head, err := repo.Head()
	require.NoError(t, err)
	revs, err := repo.RevList(head)
	require.NoError(t, err)
	assert.Len(t, revs, 3)
}

// A deepen rewrites the shallow file. A walk after it must see the parents of
// the boundary, which an earlier walk read and cached without them.
func TestADeepenedCloneShowsTheOldBoundarysParents(t *testing.T) {
	src := historyRepo(t)
	dir := filepath.Join(t.TempDir(), "shallow")
	gitT(t, src, "clone", "-q", "--depth", "1", "file://"+src, dir)
	repo := open(t, dir)
	head, err := repo.Head()
	require.NoError(t, err)
	revs, err := repo.RevList(head)
	require.NoError(t, err)
	require.Len(t, revs, 1)

	gitT(t, dir, "fetch", "-q", "--deepen=2", "origin")
	revs, err = repo.RevList(head)
	require.NoError(t, err)
	assert.Len(t, revs, 3, "the walk kept the boundary commit it cached before the deepen")
}

func TestNoRepository(t *testing.T) {
	repo, err := gitread.OpenWorkTree(t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, repo)
}

func TestAReftableRepositoryResolvesThroughGit(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	dir := t.TempDir()
	if exec.Command("git", "init", "-q", "--ref-format=reftable", dir).Run() != nil {
		t.Skip("this git writes no reftable")
	}
	writeT(t, dir, "a.md", "A.\n")
	gitT(t, dir, "add", "a.md")
	gitT(t, dir, "commit", "-q", "-m", "a")
	repo := open(t, dir)
	head, err := repo.Head()
	require.NoError(t, err)
	assert.Equal(t, gitT(t, dir, "rev-parse", "HEAD"), head.String())
	blob, err := repo.BlobAt(head, "a.md")
	require.NoError(t, err)
	assert.Equal(t, "A.\n", string(blob))
}

func TestAnUnsupportedFormatIsAnError(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "--object-format=sha256")
	_, err := gitread.OpenWorkTree(dir)
	assert.ErrorIs(t, err, gitread.ErrUnsupported)
}

//go:embed testdata/new-repo.md
var newRepoMd string
