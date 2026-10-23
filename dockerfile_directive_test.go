package slopfix_test

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

//go:embed testdata/dockerfile-with-directives.dockerfile
var dockerfileWithDirectives string

func TestFixKeepsDockerfileParserDirectives(t *testing.T) {
	for _, path := range []string{"Dockerfile", "Containerfile", "Dockerfile.runner", "app.dockerfile"} {
		t.Run(path, func(t *testing.T) {
			repair := slopfix.Fix(slopfix.Request{Content: dockerfileWithDirectives, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines})
			assert.True(t, strings.HasPrefix(repair.Text, "# syntax=docker/dockerfile:1\n# check=error=true\n"), repair.Text)
		})
	}
}
