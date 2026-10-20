#!/usr/bin/env bash
set -eo pipefail
for TAG in a b; do
  SOURCES=()
  if [ ${#SOURCES[@]} -eq 0 ]; then
    echo "::error::no per-arch image was pushed for ${TAG}"; exit 1
  fi
done
