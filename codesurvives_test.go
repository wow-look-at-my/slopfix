package slopfix

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// rustConsts is the file a Write lost consts from. Each const has its own
// doc comment, under a module doc comment.
//
//go:embed testdata/rust-consts.rs
var rustConsts string

// codeLines is every line that is neither blank nor a comment.
func codeLines(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// A repair rewrites comments. It never deletes the code they document.
func TestARepairNeverDeletesACodeLine(t *testing.T) {
	got := Fix(Request{Content: rustConsts, Path: "a.rs", MaxCommentLines: 14})
	assert.Equal(t, codeLines(rustConsts), codeLines(got.Text), "removed %q", got.Removed)
}
