package tombstones

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// foreignRepo writes files into a fresh repository and answers its root.
func foreignRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	for rel, body := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644))
	}
	return dir
}

// The unreal-tools sources port Unreal Engine formats. Their comments name
// engine types and cite the engine source by line, and no engine source is in
// the repository.
const portSrc = "package scrub\n\n" +
	"// parseARHeaderAndBatch validates the 24-byte header and walks the name batch\n" +
	"// (UnrealNames.cpp:4588-4627 archive form: Num, NumStringBytes, HashVersion,\n" +
	"// Hashes[Num], FSerializedNameHeader[Num], StringBytes).\n" +
	"func parseARHeaderAndBatch() {}\n\n" +
	"var arGUID = [4]uint32{0x717F9EE7} // FAssetRegistryVersion::GUID\n"

func TestANameFromACitedForeignSourceIsNotDead(t *testing.T) {
	dir := foreignRepo(t, map[string]string{"scrub/assetregistry.go": portSrc, "scrub/other.go": "package scrub\n"})
	path := filepath.Join(dir, "scrub", "assetregistry.go")
	assert.Empty(t, DeadReferents(path, portSrc, AddedBlocks(path, portSrc)))
}

// The test of a port names the same engine API, with no citation of its own.
func TestTheTestOfAPortNamesTheSameForeignAPI(t *testing.T) {
	test := "package scrub\n\nfunc build() {\n\thash(name) // GenerateLowerCaseHash\n}\n"
	dir := foreignRepo(t, map[string]string{"scrub/assetregistry.go": portSrc, "scrub/assetregistry_test.go": test})
	path := filepath.Join(dir, "scrub", "assetregistry_test.go")
	assert.Empty(t, DeadReferents(path, test, AddedBlocks(path, test)))
}

// A citation of a file the repository holds ports nothing, and a file with no
// citation is judged as before. A dead in-repo symbol stays dead in both.
func TestADeadInRepoSymbolStaysDeadBesideAPort(t *testing.T) {
	own := "package scrub\n\n// parseLegacyFlag reads the input (reader.c:12).\nfunc main() {}\n"
	plain := "package scrub\n\n// readLegacyInput reads the input.\nfunc other() {}\n"
	dir := foreignRepo(t, map[string]string{
		"scrub/assetregistry.go": portSrc,
		"scrub/own.go":           own,
		"cmd/plain.go":           plain,
		"scrub/reader.c":         "int main(void) { return 0; }\n",
	})
	for rel, want := range map[string]string{"scrub/own.go": "parseLegacyFlag", "cmd/plain.go": "readLegacyInput"} {
		src := map[string]string{"scrub/own.go": own, "cmd/plain.go": plain}[rel]
		path := filepath.Join(dir, rel)
		assert.Equal(t, []string{want}, DeadReferents(path, src, AddedBlocks(path, src)), rel)
	}
}

func TestTestCounterpart(t *testing.T) {
	for path, want := range map[string]string{
		"a/x_test.go": "a/x.go",
		"a/x.go":      "a/x_test.go",
		"a/x.test.ts": "a/x.ts",
		"a/x.spec.js": "a/x.js",
		"a/x.ts":      "a/x.test.ts",
	} {
		assert.Equal(t, want, testCounterpart(path), path)
	}
}
