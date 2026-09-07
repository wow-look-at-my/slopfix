# markdown

This is not a rule. It is the document model every prose rule sits on.

`Split` breaks a document into prose blocks and verbatim blocks. A fence, a table, a heading and a blank run reach the caller marked verbatim, so no rule reflows them or reads them. That split is where the counts rule, the tombstones rules and the STE rules all get their fenced-code, indented-code and list exemptions. None of them implements that exemption itself.

A block also carries its list marker and its indentation, so a nested item survives a rewrite as the item it was.

## The rule that sits on it

The hard-wrap rule is the one rule here. It lives on `Format` and `FormatFunc`, with `WordsOnly` as the proof that a rewrite moved newlines and nothing else.

### What it rejects

A paragraph split over several source lines.

### Why

The reader's window wraps a paragraph. An author's wrap freezes a guess at one window's width into the file. Every later edit to that paragraph then re-flows lines the change never touched.

### Worked example

```md
This is a wrapped
paragraph.
```

```md
This is a wrapped paragraph.
```

### What it does not flag

A verbatim block is never joined. A fence, a table and a heading each mean something by their line breaks.

A workflow and an action manifest are refused outright rather than joined. A newline there is syntax, and joining a wrapped `concurrency:` block makes GitHub reject the file before a job starts.

A rewrite whose words differ from the source is refused rather than written. Joining must only move newlines. A result that lost a word is a defect in the splitter. The caller then keeps the original.

### Repair

It repairs. Each prose block is written back as a single line, with its marker and indentation preserved.

The prose repairs run inside the same pass. A rule reads a paragraph as a sentence stream, and a hand wrap hides half of it.

### Rule ID, and running it

The ID is `wrap/hard-wrap`.

```sh
slopfix fmt doc.md
slopfix fix --only wrap doc.md
slopfix check doc.md
```

`fmt` moves newlines and nothing else. `fix` joins and repairs together.

### Which hook selects it

The `common-checks` plugin, at PreToolUse. It names no rule at all: this rule is part of the default check set, which is what that gate means.
