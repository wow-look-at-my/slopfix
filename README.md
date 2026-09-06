# slopfmt

One tool for the prose rules this org applies to a markdown file, and for the same rules a code comment must follow.

## Build

```sh
go-toolchain            # builds build/slopfmt, and runs the tests
```

## Use

```sh
slopfmt check docs/*.md   # report what the rules reject, and exit 1 when anything does
slopfmt fix docs/*.md     # repair each file in place, and report what is left
slopfmt fmt docs/*.md     # join every wrapped paragraph back to a single line
slopfmt purge .           # delete the markdown a repository must not keep
```

`fix` also reads a document on stdin and writes the repaired one on stdout. With `--json` the whole answer is one object, which is what a hook reads.

## What fix repairs

- A wrapped paragraph, joined back to a single line.
- A contraction, written out. A banned modal, replaced by the approved word.
- A semicolon and a comma splice, each replaced by the period it stands in for.
- The cardinal in a stated count, cut out. The sentence then stays true.

A sentence over the word cap is reported and left alone. To split one, the writer must know which half is the point.

## What check reports

- A paragraph split over several lines. The reader's window wraps a paragraph. An author's wrap freezes one window's width into the file.
- A semicolon, a contraction, and a banned modal. STE approves none of them.
- A comma joining clauses that each stand alone. The rule wants a subject and a finite verb after the comma.
- A sentence over the word cap.
- A stated count of items. The number is true until somebody changes the set, and nothing corrects it then.

## What no rule reads

A fenced code block, a table and a heading are data. No rule reads them. The `fmt` command never reflows them.

An inline code span and a link target hold whatever they hold. Both are masked before a rule sees the text, and `fix` leaves both as they are.

`fmt` carries a guarantee `fix` keeps: it moves newlines and nothing else. A rewrite whose words differ from the source is refused rather than written.
