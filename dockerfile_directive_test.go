package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

const dockerfileWithDirectives = `# syntax=docker/dockerfile:1
# check=error=true

# The control plane. The web UI is bundled by cmd/buildweb (esbuild via its Go
# API) and go:embed'ed, so no Node toolchain exists in this image or in CI.
FROM alpine:3.21 AS build
WORKDIR /src
COPY . .
RUN go build -o /out/app ./cmd/app
ENTRYPOINT ["/out/app"]
`

func TestFixKeepsDockerfileParserDirectives(t *testing.T) {
	for _, path := range []string{"Dockerfile", "Containerfile", "Dockerfile.runner", "app.dockerfile"} {
		t.Run(path, func(t *testing.T) {
			repair := slopfix.Fix(slopfix.Request{Content: dockerfileWithDirectives, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines})
			assert.True(t, strings.HasPrefix(repair.Text, "# syntax=docker/dockerfile:1\n# check=error=true\n"), repair.Text)
		})
	}
}
