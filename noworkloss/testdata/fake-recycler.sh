#!/bin/sh
set -eu
dir=$(dirname "$0")
case "$1" in
list) cat "$dir/items.json" ;;
restore)
	[ "$2" = broken ] && { echo "recycler: restore failed" >&2; exit 1; }
	echo "$2" >> "$dir/restored-ids"
	printf 'kept\n' > "$(jq -r --arg id "$2" '.[] | select(.id == $id) | .original_path' "$dir/items.json")"
	;;
esac
