package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A license notice above a one-line declaration outweighs the code by every
// measure. It is not a comment about the code, so no rule reads it.
func TestALicenseNoticeIsNeverMeasuredOrCut(t *testing.T) {
	src := strings.Join([]string{
		"/*",
		"** Copyright 2015-2024 The Khronos Group Inc.",
		"**",
		"** SPDX-License-Identifier: Apache-2.0",
		"*/",
		"",
		"int x;",
		"",
	}, "\n")
	assert.Empty(t, CheckLength("vk.h", src))
	fixed, changed := FixLength("vk.h", src)
	assert.False(t, changed)
	assert.Equal(t, src, fixed)
}

// A copyright line inside an ordinary doc comment still shields the block,
// because a cut from the end can reach the notice.
func TestACopyrightLineShieldsItsBlock(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// Copyright (c) 2020 Example Corp. All rights reserved.",
		"// Licensed under the MIT license. See LICENSE in the project root for",
		"// the full text of the license and the conditions that apply to it.",
		"var x = 1",
		"",
	}, "\n")
	assert.Empty(t, CheckLength("a.go", src))
	_, changed := FixLength("a.go", src)
	assert.False(t, changed)
}
