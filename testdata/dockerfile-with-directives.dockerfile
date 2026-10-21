# syntax=docker/dockerfile:1
# check=error=true

# The control plane. The web UI is bundled by cmd/buildweb (esbuild via its Go
# API) and go:embed'ed, so no Node toolchain exists in this image or in CI.
FROM alpine:3.21 AS build
WORKDIR /src
COPY . .
RUN go build -o /out/app ./cmd/app
ENTRYPOINT ["/out/app"]
