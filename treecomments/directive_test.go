package treecomments

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/edit"
)

func TestIsDirective(t *testing.T) {
	for line, want := range map[string]bool{
		"//go:nosplit":                   true,
		"\t//go:build cosmo":             true,
		"//go:linkname a b":              true,
		"// +build linux":                true,
		"//line x.go:12":                 true,
		"//export Answer":                true,
		"//extern answer":                true,
		"//sys\tFoo(fd int) (err error)": true,
		"//sysnb\tBar() (pid int)":       true,
		"//nolint:errcheck":              true,
		"// go:nosplit":                  false,
		"// The line is prose.":          false,
		"//":                             false,
		"//lines of prose":               false,
		"//TODO(x): prose":               false,
	} {
		assert.Equal(t, want, IsDirective(line), line)
	}
}

// A userscript metadata block is read by Tampermonkey, every line of it.
func TestAUserscriptMetadataLineIsADirective(t *testing.T) {
	for line, want := range map[string]bool{
		"// ==UserScript==":                         true,
		"// ==/UserScript==":                        true,
		"// @name         GitHub Actions Colorizer": true,
		"// @run-at       document-end":             true,
		"// @noframes":                              true,
		"// The @name key names the script.":        false,
		"// email me@example.com":                   false,
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

// A copyright year is churn: a yearly bump only pads a commit. The gate lets a
// repair cut it.
func TestTheGateLetsAnEditCutTheCopyrightYear(t *testing.T) {
	e := commentEdit(t, "p.go", headerSrc, "Copyright", "// Copyright The Go Authors. All rights reserved.")
	res := Apply("p.go", headerSrc, []edit.Edit{e}, edit.Scope{})
	assert.Empty(t, res.Refused)
	assert.True(t, strings.HasPrefix(res.Text, "// Copyright The Go Authors. All rights reserved.\n"))
}
