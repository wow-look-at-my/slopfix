package slopfix_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/pins"
)

// A pinned download URL is repaired in every kind of file, including a string
// in code, which no comment gate reaches.
func TestFixUnpinsADownloadURLInEveryKindOfFile(t *testing.T) {
	cases := []struct{ path, in, want string }{
		{
			path: "internal/server/dashboard/generate-timeline.sh",
			in:   "#!/bin/sh\nset -eu\ncurl -fsSL 'https://dl.pazer.build/ts0?v=10&os=linux&arch=amd64' -o .cache/ts0.cjs\n",
			want: "#!/bin/sh\nset -eu\ncurl -fsSL 'https://dl.pazer.build/ts0?arch=amd64&os=linux' -o .cache/ts0.cjs\n",
		},
		{
			path: "README.md",
			in:   "Get it from `https://dl.pazer.build/dats?v=3&os=linux&arch=amd64`.\n",
			want: "Get it from `https://dl.pazer.build/dats?arch=amd64&os=linux`.\n",
		},
		{
			path: ".github/workflows/ci.yml",
			in:   "on:\n  push:\n    branches: ['**']\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: curl -fSL \"https://dl.pazer.build/dats?os=linux&arch=amd64&v=3\" -o dats\n",
			want: "on:\n  push:\n    branches: ['**']\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: curl -fSL \"https://dl.pazer.build/dats?arch=amd64&os=linux\" -o dats\n",
		},
	}
	for _, c := range cases {
		repair := slopfix.Fix(slopfix.Request{Path: c.path, Content: c.in, Rules: []slopfix.Rule{slopfix.RulePins}})
		assert.Equal(t, c.want, repair.Text, c.path)
		for _, finding := range repair.Findings {
			assert.NotEqual(t, pins.ID, finding.ID, c.path)
		}
	}
}

// A check that writes nothing still reports the pin.
func TestCheckReportsAPinnedURL(t *testing.T) {
	path := writeFile(t, "fetch.sh", "#!/bin/sh\ncurl -fsSL 'https://dl.pazer.build/ts0?v=10&os=linux&arch=amd64' -o ts0\n")
	tree := slopfix.CheckTreeWith(filepath.Dir(path), slopfix.Request{Rules: []slopfix.Rule{slopfix.RulePins}})
	var ids []string
	for _, finding := range tree.Findings {
		ids = append(ids, finding.ID)
	}
	assert.Contains(t, ids, pins.ID)
}
