module github.com/wow-look-at-my/slopfix

go 1.26.0

require (
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.12.1
	github.com/wow-look-at-my/go-containers v0.0.0-20260826161058-40a3d1ef3d41 // go-toolchain:auto-branch
	github.com/wow-look-at-my/go-tree-sitter v0.0.0-20260908082958-768c52b963f3 // go-toolchain:auto-branch
)

replace github.com/wow-look-at-my/go-tree-sitter => /home/user/go-tree-sitter

require (
	go.yaml.in/yaml/v3 v3.0.5
	mvdan.cc/sh/v3 v3.14.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)
