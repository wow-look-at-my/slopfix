package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The docker build reads a parser directive, so no comment repair may cut it.
func TestTheRepairsLeaveDockerDirectivesAlone(t *testing.T) {
	cases := map[string]string{
		"Dockerfile":   "# syntax=docker/dockerfile:1\n# check=skip=JSONArgsRecommended\n\n# the base image\nFROM alpine\n",
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
