# commentlength

A comment longer than the code it documents. A comment earns its place by stopping the next mistake. One that outruns the code becomes an essay, and the reader pays for it on every pass through the file.

The rule is a proxy rather than a judgement of content. Length is what a machine can measure, and the org's own ruling is that a comment which needs paragraphs is a comment that needs cutting.

## What it rejects

A run of comment lines weighed against the construct beneath it. Lines catch an essay. Characters catch a dense paragraph. Either measure alone is enough to report the block. A comment inside a budget on both is left alone, however long the file it sits in.

The budget has a floor. A short comment is never a finding whatever it documents. Below that size there is nothing worth cutting.

## How the span is found

From a real syntax tree, through [go-tree-sitter](https://github.com/wow-look-at-my/go-tree-sitter). A comment is a node whose type carries `comment`, which is how every grammar spells it. The documented construct is the next named sibling. Nothing in the rule names a language.

That is what makes the repair safe. A line walk guesses where a declaration ends, and a span wrong by a line deletes the wrong prose. The rule this one replaced repaired Go alone for that reason. It had `go/ast` for Go and a guess everywhere else, so every other language got a finding nothing can act on.

A file that does not parse yields nothing. A file mid-edit is the common case for a hook, and half a tree is a worse input than none.

## What it does not flag

A comment above the package declaration. It introduces the package rather than a construct, so there is nothing of comparable size to weigh it against.

A trailing comment sharing its line with code. Measuring that line counts the code as comment text, and cutting it deletes the code.

A directive line, such as a build constraint, an interpreter line or a linter pragma. It addresses a tool rather than a reader.

## Repair

It cuts, from the end. A comment leads with its point and elaborates afterwards. The trailing paragraph is therefore what a reader loses least by losing. A paragraph goes before a line does.

The opening sentence is never cut. A block trimmed to nothing is a worse edit than a block left long. A block that cannot reach its budget without losing the opening is reported as unrepairable. It stays for a person to rewrite.

## Rule ID, and running it

The ID is `comments/length`.

```sh
slopfix comment-length .              # report
slopfix comment-length --fix .        # repair in place, and report what is left
slopfix comment-length pkg/thing.go
```

A directory is walked, skipping hidden directories and the ones holding text nobody in the tree authored. A file the rule has no grammar for is skipped.

## Languages

Go, C, C++, Rust and Bash, by file extension. Every one shares this code: the walk, the measure and the repair. A language reaches the rule by adding its grammar, and `languages_test.go` refuses a grammar that no fixture proves the repair works on.

## Which consumer selects it

go-toolchain's vet phase carries this rule. This repository's own build is the rule's first real corpus, and `selfrepair_test.go` runs it over this tree.
