package gitread_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/gitread"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func writeT(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

// newRepo builds a repository with history, a tag, a merge, attributes and ignore rules.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "-b", "main")
	writeT(t, dir, "doc.md", "# Doc\n\nAlpha.\n")
	writeT(t, dir, "sub/deep.go", "package p\n\nvar A = 1\n")
	writeT(t, dir, ".gitattributes", "isa/** linguist-vendored\nroutes.txt linguist-generated\n")
	writeT(t, dir, ".gitignore", "out.go\n")
	writeT(t, dir, "isa/ch01.go", "package p\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "base")
	gitT(t, dir, "tag", "-a", "-m", "v1", "v1")

	gitT(t, dir, "checkout", "-q", "-b", "feature")
	writeT(t, dir, "doc.md", "# Doc\n\nAlpha.\n\nBeta.\n")
	writeT(t, dir, "sub/deep.go", "package p\n\nvar A = 2\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "feature")

	gitT(t, dir, "checkout", "-q", "main")
	writeT(t, dir, "other.md", "Other.\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "main moves")
	gitT(t, dir, "merge", "-q", "--no-ff", "-m", "merge feature", "feature")
	return dir
}

func open(t *testing.T, dir string) *gitread.Repo {
	t.Helper()
	repo, err := gitread.OpenWorkTree(dir)
	require.NoError(t, err)
	require.NotNil(t, repo, "the work tree is not a repository")
	return repo
}

func TestHeadAndTags(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)

	head, err := repo.Head()
	require.NoError(t, err)
	require.False(t, head.IsZero())

	tag, err := repo.Resolve("refs/tags/v1")
	require.NoError(t, err)
	peeled, err := repo.PeelCommit(tag)
	require.NoError(t, err)
	base := gitT(t, dir, "rev-parse", "v1^{commit}")
	assert.Equal(t, base, peeled.String())

	commits, err := repo.RevList(head)
	require.NoError(t, err)
	want := strings.Fields(gitT(t, dir, "rev-list", "HEAD"))
	assert.Len(t, commits, len(want))
}

func TestMergeBase(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)
	head, err := repo.Head()
	require.NoError(t, err)
	feature, err := repo.Resolve("refs/heads/feature")
	require.NoError(t, err)
	base, ok, err := repo.MergeBase(head, feature)
	require.NoError(t, err)
	require.True(t, ok)
	want := gitT(t, dir, "merge-base", "HEAD", "feature")
	assert.Equal(t, want, base.String())
}

func TestTreeBlobAndChangedPaths(t *testing.T) {
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

	base, err := repo.Resolve("refs/tags/v1")
	require.NoError(t, err)
	tagged, err := repo.PeelCommit(base)
	require.NoError(t, err)

	names, err := repo.ChangedPaths(tagged, head)
	require.NoError(t, err)
	want := strings.Fields(gitT(t, dir, "diff", "--name-only", "--no-renames", tagged.String(), head.String()))
	assert.ElementsMatch(t, want, names)
}

func TestIndexEntries(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)
	idx, err := repo.Index()
	require.NoError(t, err)
	tracked := idx.Tracked()
	require.Contains(t, tracked, "doc.md")

	out := gitT(t, dir, "ls-files", "-s", "--", "doc.md")
	fields := strings.Fields(out)
	assert.Equal(t, fields[0], tracked["doc.md"].Mode)
	assert.Equal(t, fields[1], tracked["doc.md"].OID.String())
}

func TestAttributesAndIgnore(t *testing.T) {
	dir := newRepo(t)
	repo := open(t, dir)

	assert.True(t, gitread.AttributeSet(repo.Attr("isa/ch01.go", "linguist-vendored")))
	assert.True(t, gitread.AttributeSet(repo.Attr("routes.txt", "linguist-generated")))
	assert.False(t, gitread.AttributeSet(repo.Attr("doc.md", "linguist-vendored")))

	assert.True(t, repo.Ignore().Ignored("out.go", false))
	assert.False(t, repo.Ignore().Ignored("doc.md", false))
}

func TestLocalTags(t *testing.T) {
	dir := newRepo(t)
	bare := filepath.Join(t.TempDir(), "bare.git")
	gitT(t, dir, "clone", "-q", "--bare", dir, bare)

	commits, err := gitread.Tags(bare)
	require.NoError(t, err)
	assert.Len(t, commits, 1)
}
