# yaml/private-runner reads the repository a workflow belongs to. The built
# binary must take the visibility from the Actions event payload, the way a CI
# run gives it, and ask no network for it.
#
# Commands exec the freshly built binary as
# "${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/slopfix". Each test builds its fixture
# under the sandbox's own writable /tmp.

sandbox:
	network: false # the event payload answers the visibility, so no request goes out

tests:
	- desc: a private org repository moves each hosted Linux job to the org runners
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		repo="$(mktemp -d)"
		cd "$repo"
		git init -q .
		git remote add origin https://github.com/wow-look-at-my/private-runner-fixture
		mkdir -p .github/workflows
		printf 'name: CI\non:\n  push:\n    branches: ["**"]\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n  b:\n    runs-on: ubuntu-22.04\n    steps:\n      - run: docker build .\n' > .github/workflows/ci.yml
		printf '{"repository":{"full_name":"wow-look-at-my/private-runner-fixture","private":true}}' > "$repo/event.json"
		GITHUB_EVENT_PATH="$repo/event.json" "$slopfix" fix --only yaml/private-runner --path .github/workflows/ci.yml < .github/workflows/ci.yml
	  outputs:
		stdout:
			- "runs-on: ${{ vars.CI_RUNNER || 'ubuntu-latest' }}"
			- "runs-on: ${{ vars.CI_RUNNER_DIND || 'ubuntu-22.04' }}"

	- desc: a public org repository keeps its GitHub-hosted runner
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		repo="$(mktemp -d)"
		cd "$repo"
		git init -q .
		git remote add origin https://github.com/wow-look-at-my/public-runner-fixture
		mkdir -p .github/workflows
		printf 'name: CI\non:\n  push:\n    branches: ["**"]\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n' > .github/workflows/ci.yml
		printf '{"repository":{"full_name":"wow-look-at-my/public-runner-fixture","private":false}}' > "$repo/event.json"
		GITHUB_EVENT_PATH="$repo/event.json" "$slopfix" fix --only yaml/private-runner --path .github/workflows/ci.yml < .github/workflows/ci.yml
	  outputs:
		stdout:
			- "runs-on: ubuntu-latest"
		!stdout:
			- "vars.CI_RUNNER"
