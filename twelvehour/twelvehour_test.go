package twelvehour

import _ "embed"

import (
	"os"
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
		{"no marker, evening", "at 6:51", "at 6:51 pm"},
		{"no marker, ten", "by 10:30", "by 10:30 pm"},
		{"no marker, past midnight side", "at 5:10", "at 5:10 pm"},
		{"duration after", "a 3:45 minutes run", "a 3:45 minutes run"},
		{"duration before", "it took 3:45 total", "it took 3:45 total"},
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

func TestRewriteMarkerFollowsTheClock(t *testing.T) {
	zone := time.FixedZone("PST", -8*60*60)
	morning := time.Date(2026, 11, 1, 8, 15, 0, 0, zone)
	assert.Equal(t, "at 6:51 am", Rewrite("at 6:51", morning))
	assert.Equal(t, "at 2:00 pm", Rewrite("at 2:00", morning))
	lateNight := time.Date(2026, 11, 1, 23, 50, 0, 0, zone)
	assert.Equal(t, "at 1:20 am", Rewrite("at 1:20", lateNight))
}

func TestMessageTracksAFenceAcrossFlushes(t *testing.T) {
	dir := t.TempDir()
	zone := time.FixedZone("PST", -8*60*60)
	now := time.Date(2026, 11, 1, 18, 30, 0, 0, zone)
	flush := func(text string, final bool) string {
		out, err := Message(dir, "msg", text, final, now)
		assert.NoError(t, err)
		return out
	}
	assert.Equal(t, "at 6:51 pm\n```\n", flush("at 18:51\n```\n", false))
	assert.Equal(t, "18:51\n", flush("18:51\n", false))
	assert.Equal(t, "```\nat 7:10 pm\n", flush("```\nat 19:10\n", false))
	assert.Equal(t, "```\n", flush("```\n", true))
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

//go:embed testdata/test-rewrite.md
var testRewriteMd string

//go:embed testdata/test-rewrite-2.md
var testRewriteMd2 string
