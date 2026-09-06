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

## What check reports

- A paragraph split over several lines. The reader's window wraps a paragraph. An author's wrap freezes one window's width into the file.
- A semicolon, a contraction, and a banned modal. STE approves none of them.
- A comma joining clauses that each stand alone. The rule wants a subject and a finite verb after the comma.
- A sentence over the word cap. Text in parentheses counts as a single word, which keeps a citation from inflating the count.
- A stated count of items. The number is true until somebody changes the set, and nothing corrects it then.

## What check never reads

A fenced code block, a table and a heading are data. No rule reads them. The `fmt` command never reflows them.

An HTML entity ends in a semicolon, and an inline code span holds whatever it holds. Both are masked before a rule sees the text.
