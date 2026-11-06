package noworkloss

import _ "embed"

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/endpoint-boilerplate.js
var endpointBoilerplate string

func writeSibling(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func TestANewScriptCopyingASiblingIsRefused(t *testing.T) {
	dir := outsideTree(t)
	writeSibling(t, dir, "set-image.js", endpointBoilerplate+"model.image.sGLang.url = process.argv[2];\n")

	reason := askTool(t, "Write", dir, map[string]any{
		"file_path": filepath.Join(dir, "set-max-running.js"),
		"content":   endpointBoilerplate + "model.args[idx + 1] = process.argv[2];\n",
	})
	require.NotEmpty(t, reason)
	assert.Contains(t, reason, "set-image.js")
	assert.Contains(t, reason, "Edit tool")
}

// The same file in the same folder, sharing nothing, is a new tool.
func TestAnUnrelatedNewScriptIsAllowed(t *testing.T) {
	dir := outsideTree(t)
	writeSibling(t, dir, "set-image.js", endpointBoilerplate)

	assert.Empty(t, askTool(t, "Write", dir, map[string]any{
		"file_path": filepath.Join(dir, "resize.js"),
		"content":   testAnUnrelatedNewScriptIsAllowedJs,
	}))
}

// A port keeps its stem: bench.js becoming bench.ts is the same script.
func TestAPortToAnotherLanguageIsAllowed(t *testing.T) {
	dir := outsideTree(t)
	writeSibling(t, dir, "set-image.js", endpointBoilerplate)

	assert.Empty(t, askTool(t, "Write", dir, map[string]any{
		"file_path": filepath.Join(dir, "set-image.ts"),
		"content":   endpointBoilerplate,
	}))
}

// Shared lines in a document or a config are not a second program.
func TestNonScriptFilesAreNotCompared(t *testing.T) {
	dir := outsideTree(t)
	writeSibling(t, dir, "set-image.js", endpointBoilerplate)

	assert.Empty(t, askTool(t, "Write", dir, map[string]any{
		"file_path": filepath.Join(dir, "NOTES.md"),
		"content":   endpointBoilerplate,
	}))
}

func TestAFewCommonLinesAreAllowed(t *testing.T) {
	dir := outsideTree(t)
	writeSibling(t, dir, "set-image.js", endpointBoilerplate)

	assert.Empty(t, askTool(t, "Write", dir, map[string]any{
		"file_path": filepath.Join(dir, "whoami.js"),
		"content":   testAFewCommonLinesAreAllowedJs,
	}))
}

// Importing the modules a sibling imports is how shared code gets shared.
func TestSharedImportsAreNotACopy(t *testing.T) {
	dir := outsideTree(t)
	writeSibling(t, dir, "data-table.ts", sharedImportsSiblingTs)

	assert.Empty(t, askTool(t, "Write", dir, map[string]any{
		"file_path": filepath.Join(dir, "bar-chart.ts"),
		"content":   sharedImportsNewTs,
	}))
}

//go:embed testdata/shared-imports-sibling.ts
var sharedImportsSiblingTs string

//go:embed testdata/shared-imports-new.ts.txt
var sharedImportsNewTs string

//go:embed testdata/test-an-unrelated-new-script-is-allowed.js
var testAnUnrelatedNewScriptIsAllowedJs string

//go:embed testdata/test-a-few-common-lines-are-allowed.js
var testAFewCommonLinesAreAllowedJs string
