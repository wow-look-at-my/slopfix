package commentfix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A //go:embed var is one line, and the text it holds is the file it names.
// The comment above it is weighed against that file.
func TestACommentOverAnEmbedIsWeighedAgainstTheFile(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\nimport _ \"embed\"\n\n" +
		"// The stub stands in for the binary.\n// It fails when the env var is set.\n// It greps the file it gets.\n" +
		"//go:embed stub.sh\nvar stub string\n"
	path := filepath.Join(dir, "x.go")
	assert.NotEmpty(t, CheckLength(path, src), "with no file, the comment outweighs one var")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stub.sh"), []byte(strings.Repeat("echo the stub runs here\n", 6)), 0o644))
	assert.Empty(t, CheckLength(path, src+"\n"), "the embedded file is the code the comment documents")
}
