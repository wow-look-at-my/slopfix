#!/bin/bash
if [ -z "${STAGE2:-}" ]; then
  _stage2="$(mktemp "${TMPDIR:-/tmp}/cc-self-update.XXXXXX")"
  cp -- "${BASH_SOURCE[0]}" "$_stage2"
  STAGE2=1 exec bash "$_stage2" "$@"
fi
rm -f "${BASH_SOURCE[0]}"
