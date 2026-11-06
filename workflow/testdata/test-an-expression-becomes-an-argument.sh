#!/usr/bin/env bash
set -eo pipefail
grep -q "$1" out.txt || exit 1
echo "built $1 on $2"
echo ''"$2"''
