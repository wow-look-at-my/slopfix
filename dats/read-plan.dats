# check read-plan is the contract the marketplace plugin's tool.call module reads.
# The module sends one Bash call as JSON and runs the Read calls it gets back.
#
# Commands exec the freshly built binary as
# "${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/slopfix", the same as no-work-loss.dats.

sandbox:
	network: false

tests:
	- desc: a sed line read maps onto one Read, with the note
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		printf '%s' '{"command":"sed -n 2,3p a.txt","cwd":"/work"}' | "$slopfix" check read-plan
	  outputs:
		stdout:
			- '"reads":[{"file_path":"/work/a.txt","offset":2,"limit":2}]'
			- 'Your Bash command `sed -n 2,3p a.txt` read a file, so it did not run.'

	- desc: a tail count from the end reads the line total of the file
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		dir="$(mktemp -d)"
		printf 'a\nb\nc\nd\ne\n' > "$dir/f.txt"
		printf '{"command":"tail -n 2 f.txt","cwd":"%s"}' "$dir" | "$slopfix" check read-plan
	  outputs:
		stdout:
			- '"offset":4,"limit":2'

	- desc: hook module writes the module that calls check read-plan
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		dir="$(mktemp -d)/hooks"
		"$slopfix" hook module "$dir" >/dev/null
		ls "$dir"
		grep -c "'check', 'read-plan'" "$dir/register.ts"
	  outputs:
		stdout:
			- register.test.ts
			- register.ts
			- '1'

	- desc: a pipe runs as written
	  cmd: |
		set -eu
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		printf '%s' '{"command":"cat a.txt | jq .x","cwd":"/work"}' | "$slopfix" check read-plan
	  outputs:
		stdout:
			- '{"reads":null,"note":""}'

	- desc: stdin that is not the input object fails loud
	  exit: 1
	  cmd: |
		slopfix="${GO_TOOLCHAIN_DATS_BUILD_DIR:-$PWD/build}/slopfix"
		printf 'garbage' | "$slopfix" check read-plan
	  outputs:
		stderr:
			- 'read-plan: stdin is not a {command, cwd} object'
