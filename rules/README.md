# rules

The prose tables, in a single place. A file here is data a reader edits without reading Go. Nothing here is read at run time. `cmd/rulegen` parses this folder at generate time and writes the Go the binary runs.

## The folder is the table

Every `.xml` file here is loaded. A file declares which consumer it belongs to with the `for` attribute on its root:

| `for` | Consumer | What the entries govern |
|---|---|---|
| `numbers` | [commentfix](../commentfix/README.md) | what a comment says instead of a number |
| `english` | [commentlength](../commentlength/README.md) | what a comment says instead of filler |

A file is split by PURPOSE, not by size. A reader adding a filler word opens the filler file and sees its neighbours. The alternative is scrolling a table holding every unrelated kind of entry at once.

**Order is file name order, then document order.** Entries of a kind are collected across the whole folder in that order. The repair then applies each kind in turn. A longer phrase that must beat a shorter one therefore sits above it, in the same file or in an earlier file.

## The kinds of entry

A `<drop word= where= test=>` deletes a word that survives its own deletion.

A `<rewrite from= to= where= test= expect=>` swaps a whole phrase, lowercased, between word boundaries. So `just` never touches `adjustment`.

A `<pattern match= to= where= test= expect=>` carries a shape a phrase swap cannot express. `to` spells its groups `$1`, `$2`, the way Go's regexp expansion does. `rulegen` compiles `match` with [go-regex-compiler](https://github.com/wow-look-at-my/go-regex-compiler). The binary then runs a switch-based automaton rather than a regexp engine.

A `<flag phrase= say= test=>` names prose the rule refuses to rewrite. No single replacement is correct. A person makes the call, and `say` tells them what the repair is.

`where=` picks the surface an entry applies to. It reads `comment` for source, `message` for a rendered assistant message, and `both` when the entry reads the same either way. It defaults to both.

## Every entry is driven

`test` is the phrase as somebody writes it. `expect` is what the repair must produce. A test walks the folder and runs each entry. An entry that has stopped firing then says so rather than sitting here looking enforced.

An entry belongs here only when the swap keeps the meaning in EVERY context the phrase appears in. A wrong deletion is worse than a comment left long, because nobody reviews what a repair applied.
