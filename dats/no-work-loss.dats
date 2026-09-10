# Preservation drives git itself, and a repository carries hooks somebody else
# wrote: a pre-commit that refuses, a pre-push that asks for a password, a
# reference-transaction that hangs. A hook that can refuse the preservation
# commit decides whether the content the session is about to destroy survives.
#
# The suite runs the built binary against a real repository with a hook on
# every name the preservation path reaches. An in-process Go test can build the
# same fixture, but it runs on the host: a hook that hangs holds the test
# machine, and a hook that reaches the network reaches the real one. dats runs
# each command under bubblewrap, so a suite that drives a foreign hook is
# isolated from the machine that runs it.
#
# Commands exec the freshly built binary as
# "${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/slopfix": go-toolchain's dats phase
# stages throwaway copies under $GO_TOOLCHAIN_DATS_BUILD_DIR, while a bare
# `dats dats` from the repository root falls back to build/.
#
# The working directory is mounted read-only, so each test builds its fixture
# under the sandbox's own writable /tmp.

sandbox:
	network: false # a preservation push goes to a path on disk, never to a host

tests:
	- desc: a refusing hook does not stop the preservation commit
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		repo="$(mktemp -d)"
		cd "$repo"
		git init -q .
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		for name in pre-commit commit-msg post-commit reference-transaction pre-push; do
			printf '#!/bin/sh\necho refused >&2\nexit 1\n' > ".git/hooks/$name"
			chmod +x ".git/hooks/$name"
		done
		echo scratch > scratch.txt
		printf '{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"%s","tool_input":{"command":"rm scratch.txt"}}' "$repo" |
			"$slopfix" no-work-loss
		printf '\n'
		git log -1 --pretty=%s
		git show HEAD:scratch.txt
	  outputs:
		stdout:
			- 'preserved: rm would have lost 1 untracked file (scratch.txt)'
			- 'no-work-loss: preserved the working-tree version of 1 path(s)'
			- scratch
		!stdout:
			- '"permissionDecision":"deny"'

	- desc: a refusing pre-push hook does not stop the preservation push
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		base="$(mktemp -d)"
		git init -q --bare "$base/remote.git"
		git clone -q "$base/remote.git" "$base/work"
		repo="$base/work"
		cd "$repo"
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		git push -q origin HEAD:master
		printf '#!/bin/sh\necho refused >&2\nexit 1\n' > .git/hooks/pre-push
		chmod +x .git/hooks/pre-push
		echo scratch > scratch.txt
		printf '{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"%s","tool_input":{"command":"rm scratch.txt"}}' "$repo" |
			"$slopfix" no-work-loss
		printf '\n'
		echo "distinct-revs=$(git rev-parse HEAD origin/master | sort -u | wc -l)"
	  outputs:
		stdout:
			- and pushed before being allowed to proceed
			- distinct-revs=1
		!stdout:
			- The push failed
