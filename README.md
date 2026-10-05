# slopfix

slopfix checks and repairs text against the org's writing rules. It reads markdown prose, code comments, GitHub Actions workflows and the markdown layout of a repository.

It also answers the Claude Code hook events that the org's marketplace plugin sends it. A hook, a CI job and an editor all get the same verdict from the same binary.

## The rules

| Category | Rule IDs | Repairs |
|---|---|---|
| `repo` | `repo/agents-file`, `repo/budget`, `repo/package-scripts`, `repo/binary`, `repo/near-duplicate`, `repo/json`, `repo/xml` | all but `repo/near-duplicate`, `repo/json` and `repo/xml` |
| `wrap` | `wrap/hard-wrap`, `wrap/long-block` | yes |
| `ste` | `ste/contraction`, `ste/modal`, `ste/semicolon`, `ste/comma-splice`, `ste/sentence-length`, `ste/postdeterminer`, `ste/count` | yes, except a long sentence with no clause boundary |
| `english` | `english/comma-never` | yes |
| `ste`, warnings | `ste/instruction-length`, `ste/passive`, `ste/noun-cluster`, `ste/tense`, `ste/dictionary`, `ste/paragraph-length` | no, and a warning never fails `check` |
| `counts` | `counts/inventory-count` | yes |
| `tombstones` | `tombstones/*` | all but `tombstones/comment-volume` |
| `comments` | `comments/number`, `comments/length`, `comments/tail` | yes, except a block no cut can fit |
| `yaml` | `yaml/comment-block`, `yaml/all-builds-job`, `yaml/neutered-gate`, `yaml/env-indirection`, `yaml/push-tags`, `yaml/org-action-ref`, `yaml/concurrency` | yes, except a flow-style `push` mapping |
| `yaml`, warnings | `yaml/test-in-workflow` | no, because a `run:` script line is shell, and a warning never fails `check` |
| `pins` | `pins/download-version` | yes, except a templated URL |
| message | `laziness/punt`, `blame/deflection`, `ask/prose-decision` | no |

- `repo`: `CLAUDE.md` holds only `@AGENTS.md`. Each root file, each `CLAUDE.md` and each `claude_snippets/` file stays under `40000` characters. `fix` moves the largest sections of a long file into `docs/`. No executable is committed. No file is a near copy of another file of its name. Each JSON and XML file parses and meets the schema it names.
- `wrap` and `ste`: a paragraph is one line, and its prose follows ASD-STE100 Simplified Technical English.
- `counts`, `ste/count` and `comments/number`: a stated count goes stale when the set changes.
- `tombstones`: a comment that narrates history or argues for the diff.
- `comments`: a number in a comment, a comment longer than its code, and a comment cut off mid-thought.
- `yaml`: a comment block, a job named `all-builds`, a test in a `run:` script, and a gate under `continue-on-error`.
- message: a closing message that leaves the work undone, deflects blame, or hands the reader a decision.

A fenced code block, a table and a heading are data. No rule reads them.

## Install

The CI action downloads the published binary from buildhost. For local use, build it from this repository:

```sh
go-toolchain            # builds build/slopfix, and runs the tests
```

## Use

```sh
slopfix check .                       # report what the rules reject, exit 1 on any finding
slopfix fix .                         # repair in place, then report what is left
slopfix check --only ste docs/a.md    # narrow to a category
slopfix fix --only ste/semicolon a.md # narrow to a rule ID
slopfix check --json --path a.md < a.md      # JSON findings for text on stdin
slopfix check --message < message.txt        # judge a closing message
slopfix hook < payload.json                  # answer a Claude Code hook event
```

The commands are `check`, `hook`, `lsp`, `completion` and `help`.

- `check` runs every rule by default. `fix` is `check --fix`. A directory argument is walked.
- `--only` takes categories and rule IDs, comma separated. An unknown name is an error.
- `slopfix hook` reads the event off the payload and runs every guard that serves it.
- `slopfix lsp` is a language server on stdio. It publishes the findings for each open file that a build reads.

## In CI

```yml
- uses: actions/checkout@v4
- uses: wow-look-at-my/slopfix@master
```

The action runs `check .` with every rule. No input narrows it to some paths or some rules, and it never repairs.
## More

`AGENTS.md` holds the full reference: each rule, what it does not flag, the hook guards and the design decisions. `docs/` holds the depth of single subsystems.
