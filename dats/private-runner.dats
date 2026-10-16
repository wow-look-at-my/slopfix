# yaml/private-runner names the repository from the work tree alone. The built
# binary must write the visibility expression with no network and no event payload.
#
# Commands exec the freshly built binary as
# "${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/slopfix". Each test builds its fixture
# under the sandbox's own writable /tmp.

sandbox:
	network: false # the rule reads no visibility, so no request goes out

tests:
	- desc: an org repository picks each hosted Linux job's runner from the visibility at run time
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		repo="$(mktemp -d)"
		cd "$repo"
		git init -q .
		git remote add origin https://github.com/wow-look-at-my/private-runner-fixture
		mkdir -p .github/workflows
		printf 'name: CI\non:\n  push:\n    branches: ["**"]\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n  b:\n    runs-on: ubuntu-22.04\n    steps:\n      - run: docker build .\n  c:\n    runs-on: ${{ vars.CI_RUNNER || '"'"'ubuntu-24.04'"'"' }}\n    steps:\n      - run: make\n' > .github/workflows/ci.yml
		"$slopfix" fix --only yaml/private-runner --path .github/workflows/ci.yml < .github/workflows/ci.yml
	  outputs:
		stdout:
			- "runs-on: ${{ github.event.repository.visibility == 'public' && 'ubuntu-latest' || vars.CI_RUNNER || 'ubuntu-latest' }}"
			- "runs-on: ${{ github.event.repository.visibility == 'public' && 'ubuntu-22.04' || vars.CI_RUNNER_DIND || 'ubuntu-22.04' }}"
			- "runs-on: ${{ github.event.repository.visibility == 'public' && 'ubuntu-24.04' || vars.CI_RUNNER || 'ubuntu-24.04' }}"

	- desc: a repository of another owner keeps its GitHub-hosted runner
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		repo="$(mktemp -d)"
		cd "$repo"
		git init -q .
		git remote add origin https://github.com/someone/runner-fixture
		mkdir -p .github/workflows
		printf 'name: CI\non:\n  push:\n    branches: ["**"]\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n' > .github/workflows/ci.yml
		"$slopfix" fix --only yaml/private-runner --path .github/workflows/ci.yml < .github/workflows/ci.yml
	  outputs:
		stdout:
			- "runs-on: ubuntu-latest"
		!stdout:
			- "vars.CI_RUNNER"
