package gitread

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

// A packfile written by git is read back object for object.
func TestParsePackReadsAGitsOwnPackfile(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("beta\n"), 0o644))
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "base")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha changed\n"), 0o644))
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "change")

	cmd := exec.Command("git", "pack-objects", "--stdout", "--revs")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	cmd.Stdin = strings.NewReader("HEAD\n")
	pack, err := cmd.Output()
	require.NoError(t, err)
	require.NotEmpty(t, pack, "git wrote no packfile")

	objs, err := parsePack(pack, nil)
	require.NoError(t, err)
	require.NotEmpty(t, objs)

	head, err := ParseOID(gitIn(t, dir, "rev-parse", "HEAD"))
	require.NoError(t, err)
	commit, ok := objs[head]
	require.True(t, ok, "the head commit is in the pack")
	assert.Equal(t, TypeCommit, commit.typ)
	assert.Contains(t, string(commit.data), "tree ")

	blobOID, err := ParseOID(gitIn(t, dir, "rev-parse", "HEAD:a.txt"))
	require.NoError(t, err)
	blob, ok := objs[blobOID]
	require.True(t, ok, "the blob is in the pack")
	assert.Equal(t, "alpha changed\n", string(blob.data))
}

// A ref advertisement answers its refs and its capability line.
func TestParseRefAdvertisementReadsRefsAndCaps(t *testing.T) {
	oid := "1111111111111111111111111111111111111111"
	caps := "multi_ack_detailed side-band-64k ofs-delta filter symref=HEAD:refs/heads/main"
	var body bytes.Buffer
	writePkt(&body, "# service=git-upload-pack\n")
	body.WriteString("0000")
	writePkt(&body, oid+" HEAD\x00"+caps+"\n")
	writePkt(&body, oid+" refs/heads/main\n")
	body.WriteString("0000")

	refs, gotCaps, err := parseRefAdvertisement(body.Bytes())
	require.NoError(t, err)
	assert.Contains(t, gotCaps, "side-band-64k")
	assert.Contains(t, gotCaps, "symref=HEAD:refs/heads/main")
	assert.Equal(t, oid, refs["HEAD"].String())
	assert.Equal(t, oid, refs["refs/heads/main"].String())
	assert.Equal(t, "refs/heads/main", symrefTarget(gotCaps, "HEAD"))
}
