package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exporterSource is tampermonkey's test/harness/page.ts at EXPORTER_MODULE: a
// one-line JSDoc, then a line comment, over the constant.
const exporterSource = "import path from 'node:path';\n" +
	"\n" +
	"/** Absolute path of the exporter under test. */\n" +
	"// path.resolve (not `new URL(..., import.meta.url)`): the jsdom environment\n" +
	"// shadows the URL global, and Node's fileURLToPath rejects cross-realm URLs.\n" +
	"export const EXPORTER_MODULE = path.resolve(\n" +
	"\timport.meta.dirname,\n" +
	"\t'../../src/Claude/export-session-markdown.user.ts',\n" +
	");\n"

// A one-line JSDoc closes a sentence, so the cut keeps it and drops what follows.
func TestAOneLineJSDocIsAnOpeningTheCutKeeps(t *testing.T) {
	hits := CheckLength("page.ts", exporterSource)
	require.Len(t, hits, 1, "the control: the comment outweighs its code")
	assert.True(t, hits[0].Repairable, hits[0].Tell)

	out, changed := FixLength("page.ts", exporterSource)
	require.True(t, changed)
	assert.Contains(t, out, "/** Absolute path of the exporter under test. */\nexport const EXPORTER_MODULE")
	assert.Empty(t, CheckLength("page.ts", out))
}

// steerSource is splat-webgpu's froxel-path.ts banner over the steer constant.
const steerSource = "const GOV_FALLBACK_UNDER = 0.75;\n" +
	"// ---- continuous wall-clock steer (hybrid signal) ----------------------------\n" +
	"// Per-pass timestamps cannot see INTER-pass or submission overhead, so the steer\n" +
	"// reads the wall clock between presents and folds it into the model each frame.\n" +
	"const STEER_GAIN = 0.2;\n"

// A banner names a section, so the cut keeps the title and drops the prose under it.
func TestABannerIsAnOpeningTheCutKeeps(t *testing.T) {
	hits := CheckLength("froxel-path.ts", steerSource)
	require.Len(t, hits, 1, "the control: the comment outweighs its code")
	assert.True(t, hits[0].Repairable, hits[0].Tell)

	out, changed := FixLength("froxel-path.ts", steerSource)
	require.True(t, changed)
	assert.Contains(t, out, "// ---- continuous wall-clock steer (hybrid signal) ----")
	assert.NotContains(t, out, "Per-pass")
	assert.True(t, strings.HasSuffix(out, "const STEER_GAIN = 0.2;\n"), out)
	assert.Empty(t, CheckLength("froxel-path.ts", out))
}

// A rule of marks with no title is no sentence.
func TestARuleWithNoTitleIsNoBanner(t *testing.T) {
	assert.True(t, bannerTitle("// ---- dataset loading ----"))
	assert.True(t, bannerTitle("  # ==== setup ===="))
	assert.False(t, bannerTitle("// ---------------------------------------------------------------------------"))
	assert.False(t, bannerTitle("// a -> b"))
}
