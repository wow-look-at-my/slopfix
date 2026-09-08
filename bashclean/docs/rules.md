# cleanup-bash-cmds

## Fully silent by design

The hook never announces a rewrite -- no `systemMessage`, no `additionalContext`, ever, for any rule combination. Every rewrite emits only the replacement input plus `suppressOutput: true`, so nothing about the hook appears in the transcript. Reason: any visible hook message just gives the model something to blame for its own command mistakes.

The only observable trace of a rewrite is the executed command itself. For debugging, set `CLEANUP_BASH_CMDS_LOG` (see Logging) -- the log records every rewrite with the rules that fired. The single exception is the bans (heredoc, perl, file reads), which must return a `permissionDecisionReason` (without one the model will retry forever). They carry no `systemMessage` either.

## Before / After

Before (what the model asked for):

```bash
ls /nope 2>/dev/null
```

The command runs. The error is swallowed, and the model concludes the directory is merely empty.

After (what actually executes):

```bash
ls /nope
```

stderr stays visible: `ls: cannot access '/nope': No such file or directory`.

## perl: banned

`perl` is banned in this environment. Any command whose **effective command** is `perl` -- matched against the anchored pattern `^perl[0-9.]*$`, so `perl5.36` counts too -- is **denied**, not rewritten:

```json
{"hookSpecificOutput": {"hookEventName": "PreToolUse", "permissionDecision": "deny",
 "permissionDecisionReason": "perl is banned in this environment."}}
```

- Deny beats rewrite. The deny is logged as a `DENY` line with `reason="perl"`.
- `perl` as an **argument** or a **different command** is not denied. The walk never enters word-internal contexts, so `grep perl file` and `perlcritic file` run. `command -v perl` is a lookup, not an invocation.
- `perl` inside a **command/process substitution** (`echo $(perl -e 1)`) is a deliberate non-goal (see Non-goals) -- the same word-scoping that keeps `grep perl` safe.
- Fail-open still applies: an unparseable command passes through.

## Exactly which forms are scrubbed

Stderr redirections to /dev/null, anywhere in the command, in any of these spellings:

| Form | Example |
|------|---------|
| `2>/dev/null` | `grep x f 2>/dev/null && echo hit` |
| `2> /dev/null` | `cmd 2> /dev/null` |
| `2>>/dev/null` | `cmd 2>>/dev/null` |
| `2>> /dev/null` | `cmd 2>> /dev/null` |
| `2>'/dev/null'` | `cmd 2>'/dev/null'` |
| `2>"/dev/null"` | `cmd 2>"/dev/null"` |

Because the match is a parsed `Redirect` node (fd 2, `>` or `>>`, target exactly `/dev/null`), the old regex hazards are structurally impossible: `12>/dev/null` redirects fd 12 and stays. `2>/dev/null2` and `2>/dev/null.log` name different files and stay. `echo "try 2>/dev/null"` is a string and stays.

## Trailing `| head` / `| tail` removal

A trailing `| head ...` or `| tail ...` stage is dropped from the end of the command, repeatedly, with whatever flags and arguments it carries:

```bash
git log | head -5 | tail -2   ->   git log
cat f | grep x | head -3      ->   cat f      # grep becomes trailing next pass
a | head -2 && b | tail -3    ->   a | head -2 && b
```

Scope and guards:

- A limiting pipe on an earlier statement of a multi-line / `;`-joined script (`ls | tail -12` followed by more commands) is a deliberate part of that script and is preserved.
- **Never inside `$(...)` or `<(...)`.** `VAR=$(ls | head -1)` is functional capture, not output truncation, and is preserved.
- **Word boundaries are real.** `| headache`, `| tailscale status`, `| head5` are different commands and stay untouched (the stage's command word must be exactly `head` or `tail`).
- **Mid-pipeline stages stay.** `cmd | head -5 | wc` keeps its `head`. If a later trailing stage is stripped and `head`/`tail` becomes trailing, the next pass strips it too. That is the point.
- Strings are safe: `grep "foo | head" f` contains no pipeline.

## rm becomes recycler trash

Deletion is made **non-destructive by construction** -- not detected, not warned about, not gated on whether the file happens to be recoverable. Every `rm` is rewritten to [`recycler`](https://github.com/wow-look-at-my/recycler) `trash`, which moves each target to the platform's native recycle bin (the FreeDesktop trash can on Linux and the BSDs, `~/.Trash` on macOS, the Recycle Bin via the shell on Windows):

```bash
rm -rf build/ old.js     ->   recycler trash build/ old.js
rm file.txt              ->   recycler trash file.txt
xargs rm                 ->   xargs recycler trash
```

Getting it back:

```bash
recycler list              # what is in there, newest first (--json for scripts)
recycler restore notes.txt # put it back where it came from
```

### Why a rewrite and not a warning

There is already a control on the adjacent case: the `no-work-loss` plugin blocks `Write` on a path that exists and tells you to use `Edit`. **Delete-then-Write is the loophole around that hook** -- once the path is gone, `Write` is legal again. In the window between the delete and the Write, the only copy of that content is in the model's context. A compaction, an interruption, a rejected tool call, or a turn that ends there destroys it permanently.

A control that acts on the `Write` is already a full step too late. Nor can this lean on version control: the incident that motivated the rule happened in a scratchpad with no git at all. The only thing that helps is for `rm` to stop being able to destroy anything.

### The rewrite is unconditional and tree-wide

Unlike the trailing-noise rules.

```bash
rm -f a.js && node b.ts        ->  recycler trash a.js && node b.ts
ls | while read f; do rm "$f"; done
                               ->  ls | while read f; do recycler trash "$f"; done
clean() { rm -rf build; }      ->  clean() { recycler trash build; }
```

The edit happens per-`CallExpr` on the syntax tree, so only the `rm` segment is rewritten. A string literal that merely *contains* an `rm` is never touched:

```bash
echo "rm -rf /"           # untouched -- a string, not a command
grep "rm -rf" script.sh   # untouched
rmdir empty/              # a different command
```

`recycler` is emitted as a bare word so `PATH` resolves it. A call that is already `recycler` is never rewritten again (the transform runs to a fixpoint. An unguarded rule will recurse). The rule only ever emits `recycler trash` -- never `purge` or `empty`, which are themselves permanent deletes.

### Flag translation

`recycler trash` takes paths, so `rm`'s flags are dropped:

| `rm` flag | handling |
|---|---|
| `-r`, `-R`, `--recursive` | dropped -- `recycler trash` takes directories natively |
| `-f`, `--force` | dropped |
| `-v`, `--verbose` | dropped |
| `-i`, `-I`, `--interactive` | dropped -- prompts are meaningless here |
| `--` | dropped, targets pass positionally (re-emitted if a target is dash-leading, so `rm -- -weirdname` stays correct) |
| anything else | **DENIED** -- blocked with an explanation rather than guessing a translation |

Bundled clusters count when every letter is droppable (`-rf`, `-rfv`). An unknown letter anywhere in a cluster (`-rd`) takes the deny path, as do `--no-preserve-root`, `--one-file-system`, and `-d`.

`xargs`'s own value-taking flags (`-n N`, `-I R`, `-P N`, ...) are understood. The utility word is found correctly.

## Stdout redirects become tee

A trailing stdout file redirect on the FINAL top-level statement (rightmost `&&` / `||` member -- the same anchoring as head/tail) is rewritten into a pipe through tee. The file is still written but the output is no longer hidden from the transcript:

```bash
cmd > build.log         ->   cmd | tee build.log
cmd >> build.log        ->   cmd | tee -a build.log
a | b > out             ->   a | b | tee out
make > "$OUT" 2>err     ->   make 2>err | tee "$OUT"
```

The redirect's target word is reused verbatim, so quoting and expansions like `"$OUT"` survive. Every other redirect stays on the producer. The injected `set -o pipefail` keeps tee from masking the producer's exit status.

Exclusions (left exactly as written, deliberately):

- anything before the final statement -- `make > build.log` followed by more commands keeps its redirect (mid-script output routing is intentional)
- targets under `/dev/` -- `cmd > /dev/null` is a deliberate stdout discard and stays a discard
- process-substitution targets (`cmd > >(gzip)`)
- statements with more than one stdout file redirect (`cmd > a > b`)
- anything inside `$(...)` or `<(...)` -- `VAR=$(cmd > f)` is untouched
- non-stdout redirects (`cmd 2> err.log`, `cmd < in`)

## Docker Compose restarts become forced recreations

Every real command whose first three static words are `docker compose restart` is rewritten to use a detached forced recreation:

```bash
docker compose restart             -> docker compose up -d --force-recreate
docker compose restart api worker  -> docker compose up -d --force-recreate api worker
```

The rewrite applies anywhere commands execute, including loops, functions, subshells, and command substitutions. Trailing service names and flags, assignments, redirects, and surrounding `&&` / `||` / pipeline structure are preserved. It does not touch the separate `docker-compose` executable, text passed as arguments to another command, or opaque `sh -c` / `bash -c` strings.

## Sleep capped at 3 seconds

If every argument is a literal word that parses as a GNU sleep duration (decimal with optional `s`/`m`/`h`/`d` suffix) and the durations sum to <= 3 seconds. The command is untouched. EVERYTHING else has its whole argument list replaced with the single literal `3`:

```bash
sleep 2                  ->   sleep 2          # literal, under the cap
sleep 0.5s               ->   sleep 0.5s       # suffixes understood
sleep 30                 ->   sleep 3
sleep 1m                 ->   sleep 3          # 60s > 3s
sleep 1m 30              ->   sleep 3          # durations SUM
sleep $DELAY             ->   sleep 3          # non-literal: cannot be trusted
sleep "$(get_delay)"     ->   sleep 3
sleep infinity           ->   sleep 3
sleep                    ->   sleep 3          # zero args (an error anyway)
FOO=1 sleep 30 2>>e.log  ->   FOO=1 sleep 3 2>>e.log   # assigns/redirs kept
```

Notes:

- The cap is **per command**, not per script: `sleep 2 && sleep 2` is fine.
- `timeout 5 sleep 30` and `"sleep 30"` inside a string are word arguments, not command position, and are untouched by construction.
- The duration grammar is deliberately strict. Anything it does not recognize (including scientific notation like `sleep 1e-3`) takes the junk path and becomes `sleep 3`.

## Logging

The log file is the hook's debug channel (the transcript shows nothing). Set `CLEANUP_BASH_CMDS_LOG=/path/to/file` to append a record of every rewrite -- tagged with the rules that fired -- every deny, and every statement-count fail-open:

```
REWRITE	original="ls | grep foo"	cleaned="set -o pipefail\nls"	rules="grep,pipefail"
REWRITE	original="sleep 30; echo hi"	cleaned="set -o pipefail\nsleep 3\n:"	rules="sleep_cap,narration_remove,pipefail"
REWRITE	original="rm -rf build"	cleaned="set -o pipefail\nrecycler trash build"	rules="rm_recycle,pipefail"
DENY	original="cat <<EOF\nhi\nEOF"	reason="heredoc"
DENY	original="perl -e 1"	reason="perl"
DENY	original="cat notes.txt"	reason="file_read"
DENY	original="rm --nonsense a"	reason="rm_flag"
DENY	original="shred /tmp/x"	reason="shred"
DENY	original="find . -name '*.tmp' -delete"	reason="find_delete"
DENY	original="git rm foo"	reason="git_rm"
DENY	original="truncate -s 0 f"	reason="truncate_zero"
GUARD	original="..."	cleaned="..."	reason="stmt-count"
```

Log failures never break the hook.
