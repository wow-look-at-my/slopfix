# counts

A cardinal stated about what is HERE goes stale the moment somebody adds an item. This package finds that sentence and cuts the number out of it.

Which numbers count lives in [cardinal](../cardinal/README.md), beside the policy the comment rule reads. What is here is the document: which lines carry the page's own voice, and how a cardinal is cut out of a line.

## What it rejects

A count qualifies only when a frame and a quantity meet on the same line of prose.

The quantity is a cardinal governing a plural noun, with adjectives allowed between them.

The frame says the sentence talks about what is here. A possessive claims the things belong here, as in `this repo's plugins`. A having verb asserts possession or extent, as in `it ships hooks` or `there are sections`. A deictic points into the page, as in `the rules below`.

A quantity with no frame is ordinary technical prose. A rule that reports it fires on every page in the repository.

## Why

The number is true on the day somebody writes it. Nothing corrects it when the set changes, and no build reads a document. The next reader trusts the stale number instead of counting.

The org's ruling on this is that maintaining a count in a markdown file is not the kind of work worth doing at all. Cutting the cardinal is the whole repair. It needs no judgement.

## What it does not flag

The singular is unmatched. In English prose it is overwhelmingly a pronoun, so matching it reports far more good writing than bad. A page saying `one plugin` is still a page a single edit makes wrong. That gap is stated rather than closed.

A quantity reached through a function word is dropped. Without that filter `2 of the format drops` reads as a count of drops.

A match that continues a longer number is dropped. A version string is therefore not read as a count.

A fenced block, an indented block, a table and a heading never reach this rule. A backtick span is blanked inside its line, because a cardinal in verbatim machinery is a literal rather than the page's own claim.

A measurement inside a frame IS reported here. A budget gets raised and a suite gets slower, and the number belongs where it is enforced rather than in prose. The stale-count rule in `ste` draws that line elsewhere and exempts a unit.

## Repair

It repairs. The cardinal and the space after it are cut out, back to front, so an earlier span's offsets stay valid. `there are three sections` becomes `there are sections`, which stays true through the next commit.

## Rule ID, and running it

The ID is `counts/inventory-count`.

```sh
slopfix fix --only counts --path doc.md < doc.md
slopfix fix --only counts/inventory-count doc.md
```

## Which hook consumes it

The `no-counts-in-docs` plugin, at PreToolUse, on every tool. Its whole hook script runs `slopfix hook --only counts` against the text a write adds.
