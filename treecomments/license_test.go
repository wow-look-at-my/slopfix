package treecomments

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/edit"
)

// khronosHeader is the head of a vendored Vulkan header.
const khronosHeader = "#ifndef VULKAN_VIDEO_CODEC_H264STD_H_\n" +
	"#define VULKAN_VIDEO_CODEC_H264STD_H_ 1\n" +
	"\n" +
	"/*\n" +
	"** Copyright 2015-2024 The Khronos Group Inc.\n" +
	"**\n" +
	"** SPDX-License-Identifier: Apache-2.0\n" +
	"*/\n" +
	"\n" +
	"/* This header is generated from the Khronos Vulkan XML API Registry. */\n" +
	"int x;\n"

func TestALicenseNoticeIsNotAComment(t *testing.T) {
	got := texts(Extract("vulkan_video_codec_h264std.h", khronosHeader))
	assert.Equal(t, []string{"/* This header is generated from the Khronos Vulkan XML API Registry. */"}, got,
		"the notice block must not reach any rule; the prose comment beside it still does")
}

func TestEveryNoticeSpellingIsKept(t *testing.T) {
	for name, src := range map[string]string{
		"spdx line":     "// SPDX-License-Identifier: MIT\npackage p\n",
		"copyright (c)": "// Copyright (c) The Authors\npackage p\n",
		"copyright ©":   "// Copyright © The Authors\npackage p\n",
		"copyright year": "// Copyright 2024 The Go Authors. All rights reserved.\n" +
			"// Use of this source code is governed by a BSD-style license.\npackage p\n",
	} {
		assert.Empty(t, Extract("p.go", src), name)
	}
}

func TestProseAboutCopyrightStaysAComment(t *testing.T) {
	src := "// The copyright check reads the file header.\nfunc f() {}\n"
	assert.Equal(t, []string{"// The copyright check reads the file header."}, texts(Extract("p.go", src)),
		"the word alone, with no holder or year, is prose")
}

func TestTheGateRefusesAnEditIntoANotice(t *testing.T) {
	start := strings.Index(khronosHeader, "/*\n** Copyright")
	end := strings.Index(khronosHeader, "*/\n") + len("*/")
	e := edit.Edit{Start: start, End: end, Text: "/* */"}
	res := Apply("h.h", khronosHeader, []edit.Edit{e}, edit.Scope{})
	assert.Equal(t, khronosHeader, res.Text, "the notice must come back byte for byte")
	assert.Len(t, res.Refused, 1, "the gate must name the refused edit")
}
