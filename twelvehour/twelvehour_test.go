package twelvehour

import _ "embed"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRewrite(t *testing.T) {
	zone := time.FixedZone("PST", -8*60*60)
	now := time.Date(2026, 11, 1, 18, 30, 0, 0, zone)
	cases := []struct{ name, in, want string }{
		{"eta with zone", "around 18:51 PST", "around 6:51 pm PST"},
		{"bare eta", "at the latest 19:10.", "at the latest 7:10 pm."},
		{"midnight", "ready at 0:32", "ready at 12:32 am"},
		{"leading zero", "built at 07:32", "built at 7:32 am"},
		{"noon", "at 12:05", "at 12:05 pm"},
		{"seconds", "at 18:51:07", "at 6:51 pm"},
		{"utc clock", "an ETA of 07:32Z", "an ETA of 11:32 pm PST"},
		{"utc stamp", "started 2026-11-02T02:10:47Z", "started Nov 1 6:10 pm PST"},
		{"already 12 hour", "at 6:51 pm", "at 6:51 pm"},
		{"already 12 hour upper", "at 18:51 PM", "at 18:51 PM"},
		{"ambiguous hour", "at 6:51", "at 6:51"},
		{"file and line", "see main.go:13:45 here", "see main.go:13:45 here"},
		{"inline code", "ran `date` at `18:51`", "ran `date` at `18:51`"},
		{"url", "https://example.com/18:51", "https://example.com/18:51"},
		{"link target", "[log](https://x.dev/a:18:51)", "[log](https://x.dev/a:18:51)"},
		{"ratio", "a 16:9 frame", "a 16:9 frame"},
		{"fence", testRewriteMd, testRewriteMd2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Rewrite(c.in, now))
		})
	}
}

//go:embed testdata/test-rewrite.md
var testRewriteMd string

//go:embed testdata/test-rewrite-2.md
var testRewriteMd2 string
