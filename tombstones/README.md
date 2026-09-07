# tombstones

A tombstone is a comment describing a state the code has left, or an argument aimed at whoever reviews the diff. This package finds it and deletes the line it sits on.

This directory is a rule family rather than a single rule. Its members are tiers of the same property, and they share the comment scanner and the strip. A split leaves the scanner in a package holding no rule.

## What it rejects

Tiers, and the wording-based one is the weakest of them. That ordering is deliberate. A paraphrasing machine writes the text being judged, so a rule keyed to a phrase is a rule it eventually writes around.

The tell table matches the surface of a tombstone. Its members are a date, a change reference, a then-and-now contrast, and a former state. The rest are an address to the reviewer, a defence of the change, a quoted instruction, and a report of an experiment. The table is data, so extending this package means adding a row.

The dead-referent probe reads no wording at all. A comment naming a symbol that appears nowhere in the repository describes a tree that is gone. The probe shells out to ripgrep against the working tree the file sits in.

The volume cap is the tier no rewording defeats. A tombstone is surplus text, so an essay whose every sentence reads as true and current still fails here. The cap counts the lines of a merged comment run, and the caller sets it. It defaults to 14 lines.

## Why

The referent tier exists for a comment carrying no tell in its phrasing. A line saying `see TestDarwinStatfsToLinux for the pin`, written beside the change that deleted that test, is a tombstone no rewording makes true again.

The volume cap exists because a comment can be surplus while every sentence in it passes a wording rule. The reader pays for the whole block on every future pass through the file.

## Worked examples

A whole-line comment carrying a tell is cut out of the write.

```go
// This used to read the old table.
var x = 1 // previously a map
```

The repair deletes the first line and reports the second. The trailing comment shares a line with code, so no whole-line deletion resolves it.

```
var x = 1 // previously a map
removed: // This used to read the old table.
[tombstones/former-state] a former state: "previously"
    // previously a map
```

## What it does not flag

Only prose is judged. In source that means the comments and never the code, so a string literal holding the word `previously` is not a tombstone. In a document it means the paragraphs, with fences, HTML comments, frontmatter and indented blocks skipped, and backtick spans blanked.

A name in capitals is never a referent candidate. Neither is a short name. A candidate must also look like a symbol rather than prose: an underscore, an internal case change, or a leading capital alongside a digit. A comment about low-level work is full of names the repository does not define. Reporting those is how a guard earns the reputation that gets it turned off.

The probe returns nothing at all when it cannot answer. That covers no repository, no ripgrep, a timeout, and a search that errors. An absent answer must never read as a missing symbol. A comment putting more names than the bound to the repository is skipped for the same reason.

A file whose extension the comment scanner has no syntax for is not judged at all. Guessing at a syntax risks reading code as prose, and a verdict built on that is the kind a user turns off.

A document carries no volume cap. A long paragraph there is ordinary writing.

## Repair

It repairs what it can excise as a clean whole line, and reports the rest.

A finding is strippable only when its line is the raw source line the comment sits on, and nothing else shares that line. A trailing comment on a code line stays. So does the opening line of a block comment that shares a line with code. Neither case can be cut without guessing at a splice.

Every finding in a document is forced unstrippable. A document line is a paragraph rather than a sentence, so deleting it takes a keeper with it.

The volume cap is a judgement about a whole block rather than a span to excise, so it never strips.

A deleted line is named in the response and nowhere else. Git history already holds anything worth keeping. This package keeps no ledger.

## Rule IDs, and running them

An ID is the tell's own words, slugged. The members are `tombstones/address-to-the-reviewer`, `tombstones/change-reference`, `tombstones/comment-volume`, `tombstones/date`, `tombstones/defence-of-the-change`, `tombstones/former-state`, `tombstones/name-nothing-in-the-repository-defines`, `tombstones/quoted-instruction`, `tombstones/report-of-an-experiment` and `tombstones/then-and-now-contrast`.

```sh
slopfix fix --only tombstones --path pkg/thing.go < pkg/thing.go
slopfix fix --only tombstones/former-state --path pkg/thing.go < pkg/thing.go
slopfix fix --only tombstones --max-comment-lines 0 --path pkg/thing.go < pkg/thing.go
```

`--path` is required for this family. The path decides the comment syntax, and whether the file is judged at all.

## Which hook selects it

The `no-tombstones` plugin, at PreToolUse, on every tool. Its whole hook script runs `slopfix hook --only tombstones` with the cap passed through, against the text a write adds.
