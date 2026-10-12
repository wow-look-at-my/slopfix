package noworkloss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const endpointBoilerplate = `const NAME = "qwen38-flash-next-nvfp4";
const TOKEN = process.env.HF_TOKEN;
if (!TOKEN) throw new Error("HF_TOKEN is required");
const URL = ` + "`https://api.endpoints.huggingface.cloud/v2/endpoint/alternateraise/${NAME}`" + `;
const headers = { Authorization: ` + "`Bearer ${TOKEN}`" + `, "Content-Type": "application/json" };

const getRes = await fetch(URL, { headers });
if (!getRes.ok) throw new Error(` + "`GET ${getRes.status}: ${await getRes.text()}`" + `);
const model = (await getRes.json()).model;
`

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
		"content":   "import sharp from \"sharp\";\nconst input = process.argv[2];\nconst output = process.argv[3];\nawait sharp(input).resize(512, 512).toFile(output);\nconsole.log(`resized ${input} into ${output}`);\n",
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
		"content":   "const TOKEN = process.env.HF_TOKEN;\nif (!TOKEN) throw new Error(\"HF_TOKEN is required\");\nconst res = await fetch(\"https://huggingface.co/api/whoami-v2\", { headers: { Authorization: `Bearer ${TOKEN}` } });\nconsole.log(JSON.stringify(await res.json(), null, \"\\t\"));\n",
	}))
}
