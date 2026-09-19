# commentfix

A number stated in a comment. It is a count of what exists today. The edit that adds an item leaves it wrong.

This is the same defect the `counts` rule finds in a document, over a different substrate. Both read [cardinal](../cardinal/README.md), which holds the token walk, the vocabulary and the exemptions. What differs is the policy each substrate brings, and the two sit beside each other there. The section below says why they differ.

## What it rejects

Any number in a comment, written in digits or in letters. The cardinals, the ordinals that index a list, and the words for a repeat count all qualify.

The comment is found by the `source` package, which walks the bytes rather than parsing the language. Nothing here recompiles anything. The rule therefore answers on a tree mid-edit, before a compiler will look at it.

## Why

Nothing recompiles a comment. The stale sentence therefore survives every build, and the reader trusts it. Describing what the code does, and letting the reader count, is the repair.

## Worked example

```go
// three retries, sha256, HTTP 404, $5, §3, 10ms, 1st
package x
```

```
n.go:1:4: "three" is a number in a comment
n.go:1:52: "1" is a number in a comment
```

Everything else on that line is exempt. The reasons are below.

## What it does not flag

A generated file is skipped whole, by its own marker.

A directive line is skipped. It addresses a tool rather than a reader. It carries no prose to go stale.

An HTTP status code is a protocol answer rather than a count. The word in front of the digits is what separates it from a count wearing the same shape.

A number behind a section sign cites a section of a document. An amount carrying a currency sign is a value. The digits of a URL are part of it. A qualified name names itself.

A digit touching a letter is part of a name, so `sha256`, `amd64` and `10ms` are left alone. An ordinal suffix makes it a number again, which is why `1st` is reported above.

A file whose extension the adapter has no syntax for yields nothing. Guessing at a syntax reports a string literal as prose, and a rule nobody trusts is a rule nobody keeps.

## Divergence from the document rule, and why it stays

The document rule needs a frame as well as a quantity, and it repairs by cutting the cardinal out. This rule reports every bare number.

That is not the same rule with a looser regex. A comment is denser than a page. The org's ruling for a comment is also stricter: any number there is a count of what exists today. Folding them loosens this half, which stops a bare number being caught at all. Or it tightens the other half, which reports most ordinary prose. Each divergence traces to the substrate rather than to an implementation that learned less.

## Repair

The repair is total. What `Fix` writes carries no finding. A caller never holds a finding with no remedy behind it. It gets there in steps, each narrower than the step before it.

The table in [rules/](../rules/README.md) says the number in words wherever a swap keeps the meaning. `Say` applies it to a paragraph at a time, so a sentence that wraps across comment lines is read whole.

What no entry covers is not guessed at, because nobody reviews what a repair applied. The sentence carrying the number is cut. The caller is told which sentence went, and a comment left with nothing to say loses its line.

What neither step reaches is deleted where the check reports it. A block comment holding an indented example forms no paragraph. A rewrap destroys such an example, so the rewrite declines it. The number still goes. The example keeps its shape.

Every deletion stays inside a comment span the syntax tree named. Code is never rewritten, so a repaired file compiles exactly as it did.

To point at a section of a document, cite its slug or its heading text rather than its position. The slug survives the edit that inserts a section above it.

## The whole tree

`TreeFiles` walks a repository and answers the files a comment rule reads. It skips a hidden directory, `vendor`, `node_modules`, `testdata`, `build`, a nested module, a git submodule, and any file above a megabyte.

`FixTree` repairs every file it names, and `CheckTree` reports without writing. A repaired file is renamed into place, so a reader running beside the sweep sees a whole file either way.

The walk lives here rather than in each caller. A caller owning its own walk owns its own skip list too, and skip lists drift. Any of them ends up rewriting a vendored tree, or a submodule that belongs to somebody else.

Both readers of this walk sit outside the package. go-toolchain's comment phase runs `FixTree`. The `slopfix comments` command reads the same list.

## Rule ID, and running it

The ID is `comments/number`.

```sh
slopfix comments .
slopfix comments pkg/thing.go
```

A directory is walked, skipping hidden directories and the ones holding text nobody in the tree authored. A named file is read whatever its extension, because naming it is the request.

## Which consumer selects it

The go-toolchain build phase, which runs `slopfix comments .` over the tree. No Claude Code hook selects this rule today.
