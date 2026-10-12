package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// The package comment of bashfs internal/profiling/profiling.go, before any fix.
// Its long sentence ends on "-- using hyperfine, then exits", an -ing form after a dash.
const profilingSource = "// Package profiling bakes an optional, hyperfine-based \"profiling mode\" into\n" +
	"// packaged bashfs scripts.\n" +
	"//\n" +
	"// When a script is packaged with profiling support (the default), it gains a\n" +
	"// guarded block that does nothing on a normal run but, when the environment\n" +
	"// variable BASHFS_PROFILE_SCRIPT=1 is set, benchmarks the parts of the script\n" +
	"// that bashfs owns -- the startup integrity check and each embedded file's\n" +
	"// extraction -- using hyperfine, then exits without running the user's body.\n" +
	"//\n" +
	"// The harness itself lives in profile.sh.\n" +
	"package profiling\n"

func TestAnIngFormAfterADashBecomesAFiniteVerb(t *testing.T) {
	got := slopfix.Fix(slopfix.Request{Content: profilingSource, Path: "profiling.go"}).Text
	assert.Contains(t, got, "It uses hyperfine, then exits without running the user's body.", got)
	assert.NotContains(t, got, "It using", got)
	var left []string
	for _, f := range slopfix.CheckContent("profiling.go", got) {
		if f.ID == ste.IDSentenceCap {
			left = append(left, f.String())
		}
	}
	assert.Empty(t, left, got)
}
