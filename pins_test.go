package slopfix_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/pins"
)

// pinnedScript is a shell script with a pinned download URL, and its repair.
// Both live in testdata, which the walk skips.
func pinnedScript(t *testing.T) (string, string) {
	t.Helper()
	in, err := os.ReadFile(filepath.Join("pins", "testdata", "fetch.sh"))
	require.NoError(t, err)
	want, err := os.ReadFile(filepath.Join("pins", "testdata", "fetch.fixed.sh"))
	require.NoError(t, err)
	return string(in), string(want)
}

// A pinned download URL in a code string is repaired, which no comment gate reaches.
func TestFixUnpinsADownloadURLInAScript(t *testing.T) {
	in, want := pinnedScript(t)
	repair := slopfix.Fix(slopfix.Request{Path: "fetch.sh", Content: in, Rules: []slopfix.Rule{slopfix.RulePins}})
	assert.Equal(t, want, repair.Text)
	for _, finding := range repair.Findings {
		assert.NotEqual(t, pins.ID, finding.ID)
	}
}

// Report is what a check runs: the pin is a finding, although Fix repairs it.
func TestReportNamesThePinFixWouldRepair(t *testing.T) {
	in, _ := pinnedScript(t)
	var ids []string
	for _, finding := range slopfix.Report(slopfix.Request{Path: "fetch.sh", Content: in}).Findings {
		ids = append(ids, finding.ID)
	}
	assert.Equal(t, []string{pins.ID}, ids)
}

// A check over a tree that writes nothing still reports the pin.
func TestCheckReportsAPinnedURL(t *testing.T) {
	in, _ := pinnedScript(t)
	path := writeFile(t, "fetch.sh", in)
	tree := slopfix.CheckTreeWith(filepath.Dir(path), slopfix.Request{Rules: []slopfix.Rule{slopfix.RulePins}})
	var ids []string
	for _, finding := range tree.Findings {
		ids = append(ids, finding.ID)
	}
	assert.Contains(t, ids, pins.ID)
}

// A test asserts the exact URL the code under test produces, and a storage
// record links the version its digest covers. That pin is the point of the
// assertion, so neither the check nor the repair reaches it.
func TestAPinnedURLInATestIsAnExpectedValue(t *testing.T) {
	src := "assert.equal(record.url, 'https://dl." + "pazer.build/ue553?v=v7&os=linux&arch=amd64&debug=1');\n"
	for _, path := range []string{"test/actions/storage-record.test.ts", "web/app.spec.js", "pkg/url_test.go", "test_url.py", "src/__tests__/url.js"} {
		repair := slopfix.Fix(slopfix.Request{Path: path, Content: src, Rules: []slopfix.Rule{slopfix.RulePins}})
		assert.Equal(t, src, repair.Text, path)
		assert.Empty(t, slopfix.Report(slopfix.Request{Path: path, Content: src}).Findings, path)
		assert.Empty(t, slopfix.CheckContent(path, src), path)
	}

	repair := slopfix.Fix(slopfix.Request{Path: "lib/storage-record.ts", Content: src, Rules: []slopfix.Rule{slopfix.RulePins}})
	assert.NotContains(t, repair.Text, "v=v7", "the code that consumes a download still gets the repair")
}
