## Pattern selection

`--regex` / `--regex-file` patterns use Go RE2 syntax. Anchoring is forced. Casing is the pattern's own:

- **Patterns compile as written** (every pattern is compiled as `\A(?:pattern)\z`): matching is case-sensitive unless the pattern opts in with an inline `(?i)`. Short names are sometimes best distinguished by their case. That is the pattern author's call. The caveat: key equality IS case-insensitive, and different packages can legitimately store different casings of the same key. A case-sensitive pattern that matches only one casing rewrites some packages and misses others, so the "same" key stops matching itself across the build. When that happens by accident, the run-wide **casing-drift advisory** (below) flags it. When casings really are different parameters, nothing fires. (`--prefix` and `--keys-file` remain case-insensitive: they name keys, not text patterns.)
