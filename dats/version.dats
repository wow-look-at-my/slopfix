# version and --version print the release number and the commit go-toolchain stamped into this build.
#
# Commands exec the freshly built binary as
# "${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/slopfix", the same as no-work-loss.dats.

sandbox:
	network: false

tests:
	- desc: version prints the release number, then the full commit
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		"$slopfix" version | grep -Eqx 'slopfix [0-9]+ \([0-9a-f]{40}\)'

	- desc: --version prints the same line as version
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		test "$("$slopfix" --version)" = "$("$slopfix" version)"
