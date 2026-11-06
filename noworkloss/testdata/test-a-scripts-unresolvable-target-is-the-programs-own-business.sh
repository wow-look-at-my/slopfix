#!/bin/bash
OUT="$(dirname "$0")/../pkg/tool/$GOOS/link"
echo built > "$OUT"
rm -f "$OUT.tmp"
