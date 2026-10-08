package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
)

// txtarScript is a cmd/go script test: commands, comments and archive files.
const txtarScript = `# The first CI build of the run resolves each head and locks it. go.mod keeps
# the placeholder.
cd $WORK/m
cp go.mod $WORK/m.go.mod
env GITHUB_ACTIONS=true
go run .
stdout '^alpha-main beta-main$'
cmp go.mod $WORK/m.go.mod
exists $WORK/locks

-- m/go.mod --
module example.com/m
`

// commands answers the lines of text that are not hash comments.
func commands(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

// A script test is code. Its commands are never joined as a paragraph, and no
// prose rule reports them.
func TestAScriptTestIsNotADocument(t *testing.T) {
	for _, path := range []string{
		"testdata/script/lock.txt",
		"src/cmd/go/testdata/script/mod_lock.txt",
		"internal/x/testdata/case.txtar",
	} {
		t.Run(path, func(t *testing.T) {
			repair := slopfix.Fix(slopfix.Request{Path: path, Content: txtarScript})
			assert.Equal(t, commands(txtarScript), commands(repair.Text))
			for _, finding := range slopfix.CheckContent(path, txtarScript) {
				assert.NotEqual(t, slopfix.IDHardWrap, finding.ID)
				assert.NotEqual(t, "ste/sentence-length", finding.ID)
			}
		})
	}
}

// The negative control: a text file outside a script directory is a document.
func TestATextFileElsewhereIsStillADocument(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{Path: "notes/lock.txt", Content: txtarScript})
	assert.NotEqual(t, commands(txtarScript), commands(repair.Text))
}
