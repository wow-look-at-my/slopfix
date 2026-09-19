module github.com/wow-look-at-my/slopfix

go 1.26.0

require (
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.12.1
	github.com/wow-look-at-my/go-containers v0.0.0-20260913115023-d3bbbdd0286d // go-toolchain:auto-branch
	github.com/wow-look-at-my/go-regex-compiler v0.0.0 // go-toolchain:auto-branch
	github.com/wow-look-at-my/go-tree-sitter v0.0.0-20260914060416-7b7005e17078 // go-toolchain:auto-branch; go-toolchain:generate=a0022f830ef0
)

// rulegen runs this to compile every <pattern> in rules/ into Go. The module is
// named here because go mod tidy cannot see a go:generate line, and the run
// that writes the generated files comes before the run that imports them.
tool github.com/wow-look-at-my/go-regex-compiler/cmd/go-regex-compiler

require (
	github.com/spf13/pflag v1.0.9
	go.yaml.in/yaml/v3 v3.0.5
	mvdan.cc/sh/v3 v3.14.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
)
