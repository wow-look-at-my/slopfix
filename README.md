# slopfix

slopfix checks and repairs text against the org's writing rules. It reads markdown prose, code comments, GitHub Actions workflows and the markdown layout of a repository.

It also answers the events of the Claude Code hook that the org's marketplace plugin sends it. A hook, a CI job and an editor all get the same verdict from the same binary.

## The rules

A fenced code block, a table and a heading are data. No rule reads them.

## Install

The CI action downloads the published binary from buildhost. For local use, build it from this repository:

```sh
go-toolchain            # builds build/slopfix, and runs the tests
```

## In CI

```yml
- uses: actions/checkout@v4
- uses: wow-look-at-my/slopfix@master
```

The action runs `check .` with every rule. No input narrows it to some paths or some rules, and it never repairs.
## More

`AGENTS.md` holds the full reference: each rule, what it does not flag, the hook guards and the design decisions. `spec/`, the [slopfix-spec](https://github.com/wow-look-at-my/slopfix-spec) submodule, holds the depth of single subsystems.
