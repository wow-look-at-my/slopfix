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

## Naming the rules to run

`--only` takes a comma-separated list. An entry is a category, or a rule ID inside a category. A rule ID is the name the report prints next to the finding, the way a compiler names a warning. What a message says and what a caller asks for are the same word.

```sh
slopfmt fix --only counts            # every rule in the counts category
slopfmt fix --only ste/semicolon     # that rule alone, and no other in ste
slopfmt fix --only tombstones,wrap   # two categories
slopfmt fix --only ste/nosuch        # an error naming the rules ste holds
```

The categories are `tombstones`, `counts`, `wrap` and `ste`. A rule ID turns its category on, so naming a rule never needs the category named beside it. An unknown name is an error rather than a silent no-op, because a run that applies nothing reads as a clean file.

A word repair and the wrap join share a pass. A rule reads a paragraph as a sentence stream, and a hand wrap hides half of it. So naming a `ste` rule also joins the paragraph it repairs.

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
- A sentence over the word cap. Text in parentheses counts as a single word, which keeps a citation from inflating the count.
- A stated count of items. The number is true until somebody changes the set, and nothing corrects it then.
- A tombstone comment. It describes a state the code has left, or argues for the diff instead of telling the next editor what breaks.

## What no rule reads

A fenced code block, a table and a heading are data. No rule reads them. The `fmt` command never reflows them.

An HTML entity ends in a semicolon, and an inline code span holds whatever it holds. Both are masked before a rule sees the text, and `fix` leaves each of them as it is.

`fmt` carries a guarantee `fix` keeps: it moves newlines and nothing else. A rewrite whose words differ from the source is refused rather than written.
