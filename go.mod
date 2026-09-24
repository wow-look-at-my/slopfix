module github.com/wow-look-at-my/slopfix

go 1.26.0

require (
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.12.1
	github.com/wow-look-at-my/go-containers v0.0.0 // go-toolchain:auto-branch
	github.com/wow-look-at-my/go-tree-sitter v0.0.0 // go-toolchain:auto-branch; go-toolchain:generate=a0022f830ef0
	github.com/wow-look-at-my/xml-validator/validator v0.0.0 // go-toolchain:auto-branch
)

require (
	github.com/spf13/pflag v1.0.9
	github.com/wow-look-at-my/go-regex-compiler v0.0.0
	go.yaml.in/yaml/v3 v3.0.5
	mvdan.cc/sh/v3 v3.14.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/wow-look-at-my/xml-validator/reader v0.0.0 // indirect
)

replace mvdan.cc/sh/v3 v3.14.1 => github.com/mvdan/sh/v3 v3.14.1
