# rules

The prose tables, in a single place. A file here is data a reader edits without reading Go. The folder is embedded into the binary by `rules.FS` and loaded by `table.Load`, which collects the entries a single consumer declares and compiles every pattern.

## The folder is the table

Every `.xml` file here is loaded. A file declares which consumer it belongs to with the `for` attribute on its root:

| `for` | Consumer | What the entries govern |
|---|---|---|
| `numbers` | [commentfix](../commentfix/README.md) | what a comment says instead of a number |
| `english` | [commentlength](../commentlength/README.md) | what a comment says instead of filler |
| `ste` | [ste](../ste/README.md) | what a document says instead of a sentence the rules refuse |

A file is split by PURPOSE, not by size. A reader adding a filler word opens the filler file and sees its neighbours. The alternative is scrolling a table holding every unrelated kind of entry at once.

**Order is file name order, then document order.** Entries of a kind are collected across the whole folder in that order. The repair then applies each kind in turn. A longer phrase that must beat a shorter one therefore sits above it, in the same file or in an earlier file.

## The kinds of entry

A `<drop word= where= test=>` deletes a word that survives its own deletion.

A `<rewrite from= to= where= test= expect=>` swaps a whole phrase, lowercased, between word boundaries. So `just` never touches `adjustment`.

A `<pattern match= to= where= test= expect=>` carries a shape a phrase swap cannot express. `to` spells its groups `$1`, `$2`, the way Go's regexp expansion does, and `match` is compiled when the folder loads.

A `<flag phrase= say= test=>` names prose the rule refuses to rewrite. No single replacement is correct. A person makes the call, and `say` tells them what the repair is.

A `<case test= expect=>` is a line of prose and what the consumer writes for it, driven through the consumer as a whole rather than through one entry. It is where a phrase reaching two entries, or reaching none, is stated, and a case whose `expect` repeats its `test` names prose the consumer declines.

`where=` picks the surface an entry applies to. It reads `comment` for source, `message` for a rendered assistant message, and `both` when the entry reads the same either way. It defaults to both.

## Every entry is driven

`test` is the phrase as somebody writes it. `expect` is what the repair must produce. A test walks the folder and runs each entry. An entry that has stopped firing then says so rather than sitting here looking enforced.

An entry belongs here only when the swap keeps the meaning in EVERY context the phrase appears in. A wrong deletion is worse than a comment left long, because nobody reviews what a repair applied.
