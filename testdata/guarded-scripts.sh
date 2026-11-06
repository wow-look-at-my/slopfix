#!/usr/bin/env bash
set -eo pipefail
shopt -s nullglob
WHEELS=(dist/*.whl)
if [ ${#WHEELS[@]} -eq 0 ]; then
  echo "::error::no sglang-kernel wheel was built"; exit 1
fi
