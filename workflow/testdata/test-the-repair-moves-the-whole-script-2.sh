#!/usr/bin/env bash
set -eo pipefail
npm ci
if [ ! -f out.txt ]; then
  echo "::error::no out.txt"; exit 1
fi
