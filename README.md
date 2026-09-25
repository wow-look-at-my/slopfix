# slopfix

slopfix checks and repairs text against the org's writing rules. It reads markdown prose, code comments, GitHub Actions workflows and the markdown layout of a repository.

It also carries the Claude Code hook subcommands that the org's marketplace plugins run. A hook, a CI job and an editor all get the same verdict from the same binary.

## The rules

| Category | Rule IDs | Repairs |
|---|---|---|
| `repo` | `repo/stray-markdown`, `repo/agents-file`, `repo/budget` | the first two |
| `wrap` | `wrap/hard-wrap` | yes |
| `ste` | `ste/contraction`, `ste/modal`, `ste/semicolon`, `ste/comma-splice`, `ste/sentence-length`, `ste/postdeterminer`, `ste/count` | yes, except a long sentence with no clause boundary |
| `counts` | `counts/inventory-count` | yes |
| `tombstones` | `tombstones/*` | all but `tombstones/comment-volume` |
| `comments` | `comments/number`, `comments/length`, `comments/tail` | yes, except a block no cut can fit |
| `yaml` | `yaml/comment-block`, `yaml/all-builds-job`, `yaml/test-in-workflow`, `yaml/neutered-gate` | yes |
| message | `laziness/punt`, `blame/deflection` | no |

- `repo`: a repository keeps only `README.md`, `AGENTS.md` and `CLAUDE.md` at its root. `CLAUDE.md` holds only `@AGENTS.md`.
- `wrap` and `ste`: a paragraph is one line, and its prose follows ASD-STE100 Simplified Technical English.
- `counts`, `ste/count` and `comments/number`: a stated count goes stale when the set changes.
- `tombstones`: a comment that narrates history or argues for the diff.
- `comments`: a number in a comment, a comment longer than its code, and a comment cut off mid-thought.
- `yaml`: a comment block, a job named `all-builds`, a test in a `run:` script, and a gate under `continue-on-error`.

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
slopfix parse "The gate reads every file."
```

- `fix` is `check --fix`. A directory argument is walked.
- `--only` takes categories and rule IDs, comma separated. An unknown name is an error.
- An empty `.slopfix-spec` file at a repository root opts that repository out of `repo/stray-markdown`.
- `slopfix report --path P` prints JSON findings for text on stdin. The editor plugin reads it.
- `slopfix hook` repairs a Claude Code write. `slopfix message` judges a closing message.

## In CI

```yml
- uses: actions/checkout@v4
- uses: wow-look-at-my/slopfix@master
  with:
    paths: .                  # the default
    only: yaml/comment-block  # optional
```

The action runs `check` on `paths`. It never repairs, because a job that repairs its own checkout proves nothing.

## More

`AGENTS.md` holds the full reference: each rule, what it does not flag, the hook subcommands and the design decisions.
