package commentfix

import (
	"testing"
)

// Temporary diagnostic for the Rust doc-comment repair that empties a file.
func TestDiagRustDocBlocks(t *testing.T) {
	src := essay("///") + "const P: i32 = 1;\n"
	lines := splitLines(src)
	t.Logf("src %d bytes, %d lines", len(src), len(lines))
	for i, l := range lines {
		t.Logf("  line[%d] = %q", i, l)
	}
	bs := blocks("x.rs", src)
	t.Logf("blocks: %d", len(bs))
	for _, b := range bs {
		_, over := judge(b)
		kept := repair(b)
		t.Logf("  start=%d end=%d exact=%v codeLines=%d codeChars=%d over=%v text=%d kept=%d",
			b.start, b.end, b.exact, b.codeLines, b.codeChars, over, len(b.text), len(kept))
		for j, k := range kept {
			t.Logf("    kept[%d] = %q", j, k)
		}
	}
	out, changed := FixLength("x.rs", src)
	t.Logf("FixLength changed=%v out=%q", changed, out)
}
