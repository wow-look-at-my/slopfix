package treecomments

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/edit"
)

func TestIsDirective(t *testing.T) {
	for line, want := range map[string]bool{
		"//go:nosplit":                            true,
		"\t//go:build cosmo":                      true,
		"//go:linkname a b":                       true,
		"// +build linux":                         true,
		"//line x.go:12":                          true,
		"//export Answer":                         true,
		"//extern answer":                         true,
		"//sys\tFoo(fd int) (err error)":          true,
		"//sysnb\tBar() (pid int)":                true,
		"//nolint:errcheck":                       true,
		"// go:nosplit":                           false,
		"// The line is prose.":                   false,
		"//":                                      false,
		"//lines of prose":                        false,
		"//TODO(x): prose":                        false,
		`/// <reference types="vite/client" />`:   true,
		`/// <reference path="./globals.d.ts" />`: true,
		`/// <amd-module name="NamedModule"/>`:    true,
		"/// <summary>Prose in a tag.</summary>":  false,
		"/// The doc comment is prose.":           false,
		"//@ build-fail":                          true,
		"//@[v0] compile-flags: -C opt-level=0":   true,
		"//~^ ERROR symbol-name(_RNvXs_":          true,
		"//~| NOTE demangling(<":                  true,
		"//[v0]~^^ ERROR def-path(foo)":           true,
		"// @param prose":                         false,
		"// ~ prose":                              false,
	} {
		assert.Equal(t, want, IsDirective(line), line)
	}
}

// A userscript metadata block is read by the browser extension, every line of it.
func TestAUserscriptMetadataLineIsADirective(t *testing.T) {
	lines := []string{
		"// ==UserScript==",
		"// @name         Example Colorizer",
		"// @run-at       document-end",
		"// @noframes",
		"// ==/UserScript==",
		"// @param count how many entries to read",
		"// The @name key names the script.",
		"// email me@example.com",
	}
	want := []bool{true, true, true, true, true, false, false, false}
	assert.Equal(t, want, DirectiveMask(lines))
}

// An "// @param" line outside a metadata block is prose a rule may judge.
func TestAnAtLineOutsideAMetadataBlockIsProse(t *testing.T) {
	assert.False(t, DirectiveAt([]string{"// @param count how many entries to read"}, 0))
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
