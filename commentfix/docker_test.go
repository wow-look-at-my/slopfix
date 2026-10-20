package commentfix

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The docker build reads a parser directive, so no comment repair may cut it.
func TestTheRepairsLeaveDockerDirectivesAlone(t *testing.T) {
	cases := map[string]string{
		"Dockerfile":   testTheRepairsLeaveDockerDirectivesAloneDockerfile,
		"compose.yaml": "services:\n  app:\n    build:\n      dockerfile_inline: |\n        # syntax=docker/dockerfile:1\n\n        FROM alpine\n",
	}
	for name, src := range cases {
		assert.Equal(t, src, Fix(name, src).Text, name)
		fixed, _ := FixLength(name, src)
		assert.Equal(t, src, fixed, name)
		assert.Empty(t, Check(name, src), name)
		assert.Empty(t, CheckLength(name, src), name)
	}
}

//go:embed testdata/test-the-repairs-leave-docker-directives-alone.dockerfile
var testTheRepairsLeaveDockerDirectivesAloneDockerfile string
