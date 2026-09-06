# slopfmt

One tool for the prose rules this org applies to a markdown file, and for the same rules a code comment must follow.

## Build

```sh
go-toolchain            # builds build/slopfmt, and runs the tests
```

## Use

```sh
slopfmt check docs/*.md   # report what the rules reject, and exit 1 when anything does
slopfmt fmt docs/*.md     # join every wrapped paragraph back to a single line
slopfmt purge .           # delete the markdown a repository must not keep
```

`fix` reads text on stdin and writes the repaired text back. It is what a Claude Code hook shells out to, so the rules live here once and no plugin carries a copy.

```sh
slopfmt fix --only counts --json < notes.md
slopfmt fix --only tombstones --path loader.go --json < added.txt
```

`--only` names the rules to run: `tombstones`, `counts`, `wrap`, `ste`. `--path` names the file the text is headed for, which decides the comment syntax. The tombstone rule needs it. `--json` writes the whole answer as a single object.

## What check reports

- A paragraph split over several lines. The reader's window wraps a paragraph. An author's wrap freezes one window's width into the file.
- A semicolon, a contraction, and a banned modal. STE approves none of them.
- A comma joining clauses that each stand alone. The rule wants a subject and a finite verb after the comma.
- A sentence over the word cap. Text in parentheses counts as a single word, which keeps a citation from inflating the count.
- A stated count of items. The number is true until somebody changes the set, and nothing corrects it then.
- A tombstone comment. It describes a state the code has left, or argues for the diff instead of telling the next editor what breaks.

## What check never reads

A fenced code block, a table and a heading are data. No rule reads them. The `fmt` command never reflows them.

An HTML entity ends in a semicolon, and an inline code span holds whatever it holds. Both are masked before a rule sees the text.
