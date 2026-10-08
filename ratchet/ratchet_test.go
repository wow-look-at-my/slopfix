package ratchet_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ratchet"
)

var (
	builtOnce sync.Once
	built     string
	buildErr  error
)

// slopfixBinary builds this tree's slopfix once for every test.
func slopfixBinary(t *testing.T) string {
	t.Helper()
	builtOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ratchet-bin")
		if err != nil {
			buildErr = err
			return
		}
		built = filepath.Join(dir, "slopfix")
		buildErr = ratchet.Build("..", built)
	})
	require.NoError(t, buildErr)
	return built
}

// writeScript writes an executable shell script.
func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixer")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755))
	return path
}

// The default branch passes its own ratchet: its fix leaves every case it
// registers clean under its own check.
func TestTheTreePassesItsOwnRatchet(t *testing.T) {
	t.Serial()
	bin := slopfixBinary(t)
	failures, err := ratchet.Judge{Base: bin, Head: bin, Work: t.TempDir()}.Run(slopfix.AllRuleSpecs())
	require.NoError(t, err)
	for _, f := range failures {
		t.Error(f)
	}
}

// A branch whose fix repairs nothing fails on every rule whose case holds an
// error. That is the shape of each weakening: a rule made report-only, a cap
// raised, a detection narrowed. The rule's own detection names the errors, so
// a case the judge's check misses fails here.
func TestAFixThatRepairsNothingFailsEveryErrorRule(t *testing.T) {
	t.Serial()
	bin := slopfixBinary(t)
	head := writeScript(t, "if [ \"$2\" = --message ]; then cat; fi\n")
	specs := slopfix.AllRuleSpecs()
	failures, err := ratchet.Judge{Base: bin, Head: head, Work: t.TempDir()}.Run(specs)
	require.NoError(t, err)

	failed := set.New[string]()
	for _, f := range failures {
		failed.Add(f.Rule)
	}
	errors := 0
	for _, spec := range specs {
		if !holdsAnError(t, spec) {
			continue
		}
		errors++
		assert.True(t, failed.Contains(spec.ID), "%s: a fix that repairs nothing passed", spec.ID)
	}
	assert.NotZero(t, errors, "the control finds rules with errors to hold")
}

// holdsAnError reports whether the rule's own detection finds an error, not a
// warning, in one of its cases.
func holdsAnError(t *testing.T, spec slopfix.RuleSpec) bool {
	t.Helper()
	for _, c := range spec.Cases {
		materialized, err := slopfix.Materialize(t.TempDir(), c)
		require.NoError(t, err)
		for _, f := range spec.Detect(materialized) {
			if !f.Warning() {
				return true
			}
		}
	}
	return false
}

// A fix the judge cannot start is a failure, never a pass.
func TestAHeadThatCannotRunFails(t *testing.T) {
	t.Serial()
	bin := slopfixBinary(t)
	spec, ok := slopfix.RuleSpecByID(slopfix.IDAsk)
	require.True(t, ok)
	failures, err := ratchet.Judge{Base: bin, Head: filepath.Join(t.TempDir(), "missing"), Work: t.TempDir()}.Run([]slopfix.RuleSpec{spec})
	require.NoError(t, err)
	require.NotEmpty(t, failures)
	assert.Contains(t, failures[0].String(), slopfix.IDAsk)
}

func TestBuildNamesTheDirectoryItFailedIn(t *testing.T) {
	t.Serial()
	dir := t.TempDir()
	err := ratchet.Build(dir, filepath.Join(dir, "out"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), dir)
}
