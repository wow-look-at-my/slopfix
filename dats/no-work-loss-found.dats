# Found work drives git itself: it commits and pushes work only this machine
# holds before a command sets that work aside. Each test builds a remote and a
# clone under the sandbox's own writable /tmp, and drives the built binary.
#
# Commands exec the freshly built binary as
# "${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/slopfix", the same as no-work-loss.dats.

sandbox:
	network: false # a push goes to a path on disk, never to a host

tests:
	- desc: worktree creation from a clean, pushed repository passes unchanged
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		base="$(mktemp -d)"
		git init -q --bare -b master "$base/remote.git"
		git clone -q "$base/remote.git" "$base/work"
		repo="$base/work"
		cd "$repo"
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		git push -q origin HEAD:master
		git branch -q -u origin/master
		out="$(printf '{"hook_event_name":"PreToolUse","session_id":"clean","tool_name":"Bash","cwd":"%s","tool_input":{"command":"git worktree add ../wt -b feature"}}' "$repo" |
			"$slopfix" hook --only no-work-loss)"
		echo "out=[$out]"
		git worktree add -q ../wt -b feature
		echo "branch=$(git rev-parse --abbrev-ref HEAD)"
		echo "log=$(git log --oneline | wc -l | tr -d ' ')"
	  outputs:
		stdout:
			- out=[]
			- branch=master
			- log=1

	- desc: a worktree inside its repository passes unchanged and is excluded
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		base="$(mktemp -d)"
		git init -q --bare -b master "$base/remote.git"
		git clone -q "$base/remote.git" "$base/work"
		repo="$base/work"
		cd "$repo"
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		git push -q origin HEAD:master
		git branch -q -u origin/master
		out="$(printf '{"hook_event_name":"PreToolUse","session_id":"inside","tool_name":"Bash","cwd":"%s","tool_input":{"command":"git worktree add .claude/worktrees/feat -b feat"}}' "$repo" |
			"$slopfix" hook --only no-work-loss)"
		echo "out=[$out]"
		git worktree add -q .claude/worktrees/feat -b feat
		cat .git/info/exclude
		echo "status=[$(git status --porcelain)]"
	  outputs:
		stdout:
			- out=[]
			- /.claude/worktrees/
			- status=[]

	- desc: worktree creation from a repository with found work commits and pushes it first
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		base="$(mktemp -d)"
		git init -q --bare -b master "$base/remote.git"
		git clone -q "$base/remote.git" "$base/work"
		repo="$base/work"
		cd "$repo"
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		git push -q origin HEAD:master
		echo 'package a // edited' > tracked.go
		echo scratch > scratch.txt
		printf '{"hook_event_name":"PreToolUse","session_id":"dirty","tool_name":"Bash","cwd":"%s","tool_input":{"command":"git worktree add ../wt -b feature"}}' "$repo" |
			"$slopfix" hook --only no-work-loss
		printf '\n'
		echo "branch=$(git rev-parse --abbrev-ref HEAD)"
		echo "status=[$(git status --porcelain)]"
		git -C "$base/remote.git" show no-work-loss/dirty:scratch.txt
	  outputs:
		stdout:
			- found work in
			- branch=no-work-loss/dirty
			- status=[]
			- scratch
		!stdout:
			- '"permissionDecision":"deny"'

	- desc: a stash stores nothing, because the work it would hide is pushed first
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		base="$(mktemp -d)"
		git init -q --bare -b master "$base/remote.git"
		git clone -q "$base/remote.git" "$base/work"
		repo="$base/work"
		cd "$repo"
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		git push -q origin HEAD:master
		git switch -q -c topic
		git push -q -u origin topic
		echo 'package a // edited' > tracked.go
		printf '{"hook_event_name":"PreToolUse","session_id":"stash","tool_name":"Bash","cwd":"%s","tool_input":{"command":"git stash"}}' "$repo" |
			"$slopfix" hook --only no-work-loss
		printf '\n'
		git stash
		echo "stashes=$(git stash list | wc -l | tr -d ' ')"
		echo "remote=$(git -C "$base/remote.git" show topic:tracked.go)"
	  outputs:
		stdout:
			- found work in
			- stashes=0
			- remote=package a // edited

	- desc: a status whose output showed found work commits and pushes it
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		base="$(mktemp -d)"
		git init -q --bare -b master "$base/remote.git"
		git clone -q "$base/remote.git" "$base/work"
		repo="$base/work"
		cd "$repo"
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		git push -q origin HEAD:master
		git switch -q -c topic
		git push -q -u origin topic
		echo scratch > scratch.txt
		printf '{"hook_event_name":"PostToolUse","session_id":"seen","tool_name":"Bash","cwd":"%s","tool_input":{"command":"git status --short"},"tool_response":{"stdout":"?? scratch.txt\\n","stderr":""}}' "$repo" |
			"$slopfix" hook --only no-work-loss
		printf '\n'
		echo "remote=$(git -C "$base/remote.git" show topic:scratch.txt)"
	  outputs:
		stdout:
			- '"hookEventName":"PostToolUse"'
			- found work in
			- remote=scratch

	- desc: a stop pushes the found work of every repository the session touched
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		base="$(mktemp -d)"
		git init -q --bare -b master "$base/remote.git"
		git clone -q "$base/remote.git" "$base/work"
		repo="$base/work"
		cd "$repo"
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		git push -q origin HEAD:master
		git switch -q -c topic
		git push -q -u origin topic
		printf '{"hook_event_name":"PostToolUse","session_id":"stop","tool_name":"Read","cwd":"%s","tool_input":{"file_path":"%s/tracked.go"}}' "$repo" "$repo" |
			"$slopfix" hook --only no-work-loss
		echo 'package a // edited' > tracked.go
		printf '{"hook_event_name":"Stop","session_id":"stop","cwd":"%s"}' "$repo" |
			"$slopfix" hook --only no-work-loss
		printf '\n'
		echo "remote=$(git -C "$base/remote.git" show topic:tracked.go)"
	  outputs:
		stdout:
			- found work in
			- remote=package a // edited
		!stdout:
			- '"decision":"block"'

	- desc: a stop with found work and no remote is refused with the commands that finish the job
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		repo="$(mktemp -d)"
		cd "$repo"
		git init -q -b topic .
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		printf '{"hook_event_name":"PostToolUse","session_id":"noremote","tool_name":"Read","cwd":"%s","tool_input":{"file_path":"%s/tracked.go"}}' "$repo" "$repo" |
			"$slopfix" hook --only no-work-loss
		printf '{"hook_event_name":"Stop","session_id":"noremote","cwd":"%s"}' "$repo" |
			"$slopfix" hook --only no-work-loss
	  outputs:
		stdout:
			- '"decision":"block"'
			- has no origin remote
			- push -u origin HEAD
		!stdout:
			- stash

	- desc: a git add of an undeclared nested repository drops it and pushes its work
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		base="$(mktemp -d)"
		git init -q --bare -b master "$base/remote.git"
		git init -q --bare -b master "$base/nested.git"
		git clone -q "$base/remote.git" "$base/work"
		repo="$base/work"
		cd "$repo"
		git config user.email dats@example.com
		git config user.name dats
		echo 'package a' > tracked.go
		git add -A
		git commit -qm initial
		git push -q origin HEAD:master
		git switch -q -c topic
		git push -q -u origin topic
		git clone -q "$base/nested.git" nested
		git -C nested config user.email dats@example.com
		git -C nested config user.name dats
		git -C nested switch -q -c topic
		echo inner > nested/inner.txt
		printf '{"hook_event_name":"PreToolUse","session_id":"embedded","tool_name":"Bash","cwd":"%s","tool_input":{"command":"git add nested tracked.go"}}' "$repo" |
			"$slopfix" hook --only no-work-loss
		printf '\n'
		cat .git/info/exclude
		echo "nested-remote=$(git -C "$base/nested.git" show topic:inner.txt)"
	  outputs:
		stdout:
			- '"updatedInput":{"command":"git add tracked.go"}'
			- /nested/
			- nested-remote=inner
