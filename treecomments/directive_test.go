package treecomments

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/edit"
)

func TestIsDirective(t *testing.T) {
	for line, want := range map[string]bool{
		"//go:nosplit":            true,
		"\t//go:build cosmo":      true,
		"//go:linkname a b":       true,
		"// +build linux":         true,
		"//line x.go:12":          true,
		"//export Answer":         true,
		"//extern answer":         true,
		"//sys\tFoo(fd int) (err error)": true,
		"//sysnb\tBar() (pid int)": true,
		"//nolint:errcheck":       true,
		"// go:nosplit":           false,
		"// The line is prose.":   false,
		"//":                      false,
		"//lines of prose":        false,
		"//TODO(x): prose":        false,
	} {
		assert.Equal(t, want, IsDirective(line), line)
	}
}

const headerSrc = "// Copyright 2026 The Go Authors. All rights reserved.\n" +
	"// Use of this source code is governed by a BSD-style\n" +
	"// license that can be found in the LICENSE file.\n\n" +
	"package p\n\n" +
	"// Answer returns the answer.\n" +
	"//\n" +
	"//go:nosplit\n" +
	"func Answer() int { return 42 }\n"

func TestTheLicenseHeaderIsFound(t *testing.T) {
	start, end, ok := LicenseHeader("p.go", headerSrc)
	require.True(t, ok)
	assert.Equal(t, 0, start)
	assert.True(t, strings.HasSuffix(headerSrc[:end], "LICENSE file."))
}

func TestAHeaderWithNoNoticeIsNotALicense(t *testing.T) {
	_, _, ok := LicenseHeader("p.go", "// Package p answers.\npackage p\n")
	assert.False(t, ok)
}

// A space after the marker turns a directive off, so the gate refuses it.
func TestTheGateRefusesAnEditThatDisablesADirective(t *testing.T) {
	at := strings.Index(headerSrc, "// Answer")
	end := strings.Index(headerSrc, "\nfunc Answer")
	e := edit.Edit{Start: at, End: end, Text: "// Answer returns it.\n//\n// go:nosplit"}
	res := Apply("p.go", headerSrc, []edit.Edit{e}, edit.Scope{})
	assert.NotEmpty(t, res.Refused)
	assert.Equal(t, headerSrc, res.Text)
}

func TestTheGateLetsAnEditKeepADirective(t *testing.T) {
	e := commentEdit(t, "p.go", headerSrc, "Answer returns", "// Answer returns it.")
	res := Apply("p.go", headerSrc, []edit.Edit{e}, edit.Scope{})
	assert.Empty(t, res.Refused)
	assert.Contains(t, res.Text, "// Answer returns it.\n//\n//go:nosplit\nfunc Answer()")
}

// A license header is a legal notice: its year and its line breaks stay.
func TestTheGateRefusesAnEditToTheLicenseHeader(t *testing.T) {
	for name, text := range map[string]string{
		"the year":       "// Copyright The Go Authors. All rights reserved.",
		"a reflow":       "// Copyright 2026 The Go Authors. All rights reserved. Use of this",
	} {
		e := commentEdit(t, "p.go", headerSrc, "Copyright", text)
		res := Apply("p.go", headerSrc, []edit.Edit{e}, edit.Scope{})
		assert.NotEmpty(t, res.Refused, name)
		assert.Equal(t, headerSrc, res.Text, name)
	}
}
