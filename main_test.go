package slopfix

import (
	"os"
	"testing"
)

// Every test that judges a workflow asks branch-pin lookups of the fixture API, never of GitHub.
func TestMain(m *testing.M) {
	if err := serveFixtureRefs(nil); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
