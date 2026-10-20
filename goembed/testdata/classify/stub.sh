#!/bin/sh
if [ -n "$SLOPFIX_STUB_MISSING" ]; then echo "sh: slopfix: not found" >&2; exit 127; fi
shift
if [ "$1" = --only ]; then
  if grep -n SLOP "$3"; then exit 1; fi
  exit 0
fi
if [ "$1" = --fix ]; then
  git grep -l SLOP | while read -r f; do sed -i 's/SLOP/prose/g' "$f"; done
fi
