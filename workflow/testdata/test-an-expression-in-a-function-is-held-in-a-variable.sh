#!/usr/bin/env bash
set -eo pipefail
slopfix_arg_1="$1"
set --
assert_sha() { grep -q "${slopfix_arg_1}" "$1" || exit 1; }
assert_sha out.txt
