# version and --version print the release number and the commit go-toolchain stamped into this build.
#
# Commands exec the freshly built binary as
# "${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/slopfix", the same as no-work-loss.dats.
#
# go-toolchain sets GOSMOPOLITAN_RELEASE and GOSMOPOLITAN_RELEASE_REVISION in its
# own environment when it stamps a release, and suite commands inherit it. A CI
# build always stamps one. A local build without GO_TOOLCHAIN_RELEASE_VERSION
# stamps none, and its binary refuses to name a version.

sandbox:
	network: false

tests:
	- desc: version prints the stamped release number and commit, or refuses on an unstamped build
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		if [ -n "${GOSMOPOLITAN_RELEASE:-}" ]; then
			test "$("$slopfix" version)" = "slopfix $GOSMOPOLITAN_RELEASE ($GOSMOPOLITAN_RELEASE_REVISION)"
			"$slopfix" version | grep -Eqx 'slopfix [0-9]+ \([0-9a-f]{40}\)'
			exit 0
		fi
		if [ "${GITHUB_ACTIONS:-}" = true ]; then
			echo "a CI build stamped no release" >&2
			exit 1
		fi
		if out=$("$slopfix" version 2>&1); then
			echo "an unstamped binary named a version: $out" >&2
			exit 1
		fi
		test "$out" = "Error: this slopfix binary carries no release number: go-toolchain stamps the build setting release into every CI build"

	- desc: --version answers the same as version
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		if flag=$("$slopfix" --version 2>&1); then flagstatus=0; else flagstatus=$?; fi
		if sub=$("$slopfix" version 2>&1); then substatus=0; else substatus=$?; fi
		test -n "$flag"
		test "$flag" = "$sub"
		test "$flagstatus" = "$substatus"
