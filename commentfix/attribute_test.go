package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const attributedBody = "pub unsafe extern \"C\" fn converter_free(handle: *mut Handle) {\n" +
	"    if handle.is_null() {\n" +
	"        return;\n" +
	"    }\n" +
	"    let owned = Box::from_raw(handle);\n" +
	"    owned.close();\n" +
	"    drop(owned);\n" +
	"}\n"

const attributedDoc = "/// Free a converter handle\n" +
	"///\n" +
	"/// The handle must be null or a pointer the create call returned,\n" +
	"/// and the caller must not free it a second time.\n"

// A comment above an attributed item documents the item. Weighed against the
// attribute line alone, every doc comment over an attribute read as an essay.
func TestACommentAboveAnAttributeIsWeighedAgainstTheItem(t *testing.T) {
	for _, attrs := range []string{"#[no_mangle]\n", "#[no_mangle]\n#[inline(never)]\n"} {
		src := attributedDoc + attrs + attributedBody
		assert.Empty(t, CheckLength("x.rs", src), "%q", attrs)
		fixed, changed := FixLength("x.rs", src)
		assert.False(t, changed)
		assert.True(t, strings.HasPrefix(fixed, attributedDoc))
	}
}
