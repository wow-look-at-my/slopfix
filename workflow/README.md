# workflow

These are substrate rules. They are about the format rather than about the prose inside it. The format is a GitHub Actions workflow or an action manifest, and the prose rules never run on either.

## A family, and why it is not split further

The members share the predicate that decides which files they read at all. A reader needs that predicate before any member makes sense. A directory per member leaves the predicate in a package holding no rule. Each member still selects on its own by ID, and each has its own CI step in the org.

A file is read when its base name is `action.yml` or `action.yaml`, or when it is a YAML file under a `workflows` directory. A caller already inside the directory can also sniff the content: a left-margin `jobs:` or `runs:` key names the file for what it is.

## The members

`yaml/comment-block` rejects a run of comment lines past the limit. The limit is a single line. It takes no input. A blank line neither counts toward a run nor ends one, because a reader sees the same paragraph either way.

`yaml/all-builds-job` rejects a job named `all-builds`, by its key or by its rendered name. The org's required gate is a commit status posted by the required-builds-manager app. A job wearing the name satisfies nothing, and it shadows the real gate in the GitHub UI.

`yaml/test-in-workflow` rejects a test written into a `run:` script. An assertion pairing a comparison with a nonzero exit counts. So does a shell function whose name says it asserts, and a redirect naming a test file.

`yaml/neutered-gate` rejects a step that runs a gate under `continue-on-error`. A step allowed to fail is not a gate, and nothing in the gate's own output shows that it was switched off.

## Why

The comment-block rule has a real incident behind it. A session put a three-line YAML comment above a trigger. That failed the org's gate and took a pull request red. The same lines went into a second repository within the hour. A finding nobody reads stops nothing.

The all-builds rule carries the operator's own wording, which the code says plainly must not be softened. A job wearing the required status's name is a known deception attempt.

The tests rule exists because a workflow step is a scheduler rather than a test framework. An assertion living in YAML cannot be run by an engineer without pushing a commit.

## Worked example

```yaml
name: CI
# one
# two
jobs:
  all-builds:
    runs-on: ubuntu-latest
    steps:
      - uses: wow-look-at-my/slopfix@master
        continue-on-error: true
      - run: |
          grep -q x out || exit 1
```

`slopfix workflows .` reports the comment run against the line it opens on. It reports the job by its key. It reports the neutered step by the line the step opens on, and the assertion by the line inside the script.

## What it does not flag

Unparseable YAML yields no all-builds finding at all. A file this rule cannot read is a file the runner cannot read either. It fails on its own.

A job name holding an expression is skipped. The name resolves at run time, so no file can judge it. A matrix suffix in parentheses and a reusable workflow's path parts are stripped before the comparison. A segment must then match exactly, so a job called `all-builds2` carries a different name.

A step that merely runs a command is not a test. It fails on its own exit code. An error annotation by itself is a report rather than an expectation.

Nothing here is reflowed. A newline in a workflow is syntax, and joining a wrapped `concurrency:` block makes GitHub reject the file before a job starts. `slopfix fmt` refuses one outright.

## Repair

These rules report only. Every member names a change a person must make: shorten the comment, rename the job, move the assertion into the repository suite, or remove `continue-on-error`. None of that is a rewrite a tool can perform safely.

## Running a member

```sh
slopfix workflows .
slopfix workflows . --only yaml/comment-block
slopfix check .github/workflows/ci.yml
```

There is no way to exempt a path. The walk skips this repository's registered submodules, which carry their own CI, and that skip is derived from `.gitmodules` and verified against the index. Nothing a caller writes widens it.

A walk selecting no file exits non-zero. A run that read nothing enforced nothing, and in CI that means the step ran ahead of the checkout.

## Which hook selects it

The `common-checks` plugin, at PreToolUse. It names no rule at all: this family is part of the default check set, which is what that gate means.
