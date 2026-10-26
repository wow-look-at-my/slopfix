# clean-bash

`clean-bash` rewrites a Bash command rather than refusing it, wherever a rewrite exists. The rules run on the parse tree to a fixed point. An unparseable command passes through. A rewrite is silent: it emits only the new input and `suppressOutput: true`. A visible message gives the model something to blame. A denial carries a reason.

Without one, the model retries forever. `CLEANUP_BASH_CMDS_LOG=/path` logs each rewrite and each deny.

Denials: `heredoc`, `perl` (`^perl[0-9.]*$` as the effective command), `shred`, `git_rm` with `--pathspec-from-file`, `truncate_zero` with an unknown flag, `rm_flag` for an `rm` flag it cannot drop. And `toolchain_capture` for `go-toolchain` inside a substitution.

`cat`, `head`, `tail` and `sed` run as written. `readplan.go` maps one plain read onto Read calls, and `check read-plan` prints that mapping for one command.

- `devnull` removes `2>/dev/null` in any spelling. It matches a parsed redirect. A `12>/dev/null` thus stays.
- `rm_recycle`, `truncate_recycle` and `find_delete_recycle` turn deletion into `recycler trash`. Delete-then-Write is the loophole around the Write refusal.
- `git_rm_recycle` writes `git rm` as `git rm --cached` and then `recycler trash` on the same paths, inside each `-C` directory. A dry run passes through.
- `grep`, `or_true` and `stderr_merge` drop a trailing `| grep`, `|| true` or `2>&1` from the last statement.
- `tee` turns a trailing stdout redirect on the last statement into `| tee file`.
- `toolchain_output` strips every pipe stage or stdout redirect from `go-toolchain`.
- `docker_compose_restart` writes `docker compose up -d --force-recreate`.
- `grep_json` turns a `grep`, `egrep`, `fgrep` or `rg` over JSON files into `jq`. A fixed head and tail of each file decide, not the extension. JSON Lines prints each matching record. A document prints `.path = value` for each matching leaf. An unknown flag or a non-JSON operand leaves the grep alone. `bashclean/grepjson.xml` holds the programs, their flags, the fixtures and the tests beside each entry.
- `gh_wait_ci` maps `gh run view`, `watch`, `rerun`, `list` and `gh pr checks` to `gh wait-ci`.
- `sleep_cap` writes `sleep 3` for any sleep past `3` seconds or not literal.
- `narration_remove` drops an `echo` or a `printf` of constant text that reaches the terminal from its list or chain. In a pipeline, or where its status decides a condition, it becomes `:`. A command whose only work is narration stays as written.
- `noop_remove` drops a bare `true` or `:` from a list or chain that does real work. A no-op stays where nothing else runs, as a pipeline stage. Those stays are as the last statement of a nested list, and before a read of `$?`. A `false` stays, because it fails the list.
- `pipefail` injects `set -o pipefail` when the command has a pipe. A `tee` then never masks a failure. A command with no pipe stays one command, so a permission rule still matches it.
