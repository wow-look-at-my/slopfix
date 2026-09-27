package slopfix_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/expect"
)

// annotated joins lines, each with the annotation its pair names. The marker
// is joined at run time, so this file carries no annotation of its own.
func annotated(lines ...[2]string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l[0])
		if l[1] != "-" {
			b.WriteString(" // " + expect.Marker + " " + strconv.Quote(l[1]))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// gitlinks is a field comment the length cut once left as `one "commit.`, a
// quote opened and never closed.
func gitlinks(want string) string {
	return annotated(
		[2]string{"package codehost", "-"},
		[2]string{"", "-"},
		[2]string{"type Origin struct {", "-"},
		[2]string{"\t// Gitlinks records the commit each submodule at Hash points at, one", want},
		[2]string{"\t// \"commit path\" line per submodule, paths relative to the repo root.", ""},
		[2]string{"\t// It lives here rather than in the module zip, whose hash go.sum pins.", ""},
		[2]string{"\tGitlinks string `json:\",omitempty\"`", "-"},
		[2]string{"}", "-"},
	)
}

func TestTheGitlinksCommentIsCutAtAClause(t *testing.T) {
	src := gitlinks("\t// Gitlinks records the commit each submodule at Hash points at.")
	repair := slopfix.Fix(slopfix.Request{Path: "codehost.go", Content: src})
	assert.Empty(t, repair.Unmet)
	assert.False(t, repair.Changed, "an annotated file is never rewritten")
	assert.Equal(t, src, repair.Text)
}

func TestAnAnnotationTheRepairBreaksIsUnmet(t *testing.T) {
	src := gitlinks("\t// something slopfix does not write.")
	repair := slopfix.Fix(slopfix.Request{Path: "codehost.go", Content: src})
	require.NotEmpty(t, repair.Unmet)
	assert.Equal(t, src, repair.Text)
}

// A file on disk that breaks its annotations is an error, and is left as it was.
func TestFixFileRefusesAnUnmetFixture(t *testing.T) {
	src := gitlinks("\t// something slopfix does not write.")
	path := filepath.Join(t.TempDir(), "codehost.go")
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))

	_, err := slopfix.FixFile(path)
	var unmet *slopfix.UnmetError
	require.ErrorAs(t, err, &unmet)
	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, src, string(after))
}
