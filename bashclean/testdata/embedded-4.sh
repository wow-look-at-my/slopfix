cd /Users/mhaynie/repos/splat/splat-vulkan
PRE=/private/tmp/claude-501/-Users-mhaynie-repos-splat/10878a52-8108-4974-ae5c-3ecf2bb62d3f/scratchpad/premerge-wt/splat-vulkan/build/test-out/t5b
POST=build/test-out/t5b
echo "=== files present ==="; ls -1 "$PRE" "$POST" 2>&1 | tail -12
echo "=== REF: pre-merge vs post-merge ==="; ./build/splat-compare "$PRE/odd-size-ref.png" "$POST/odd-size-ref.png" --within 2 --pct 99 --mean 0.5
echo "=== FROXEL: pre-merge vs post-merge ==="; ./build/splat-compare "$PRE/odd-size-froxel.png" "$POST/odd-size-froxel.png" --within 2 --pct 99 --mean 0.5