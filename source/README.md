# source

This is not a rule. It is a substrate adapter.

It answers a single question: where is the prose in this file. For a source file that means the comments, and nothing else.

## Why it is separate

A prose rule is about prose. A code comment, a commit message and a plain text file all carry prose exactly as a document does. What differs between them is where the prose sits. That is the only thing an adapter decides.

Keeping the answer here means a prose rule gains a substrate by an adapter being written, rather than by the rule being written again.

The extractor was called `gocomments` before. Only the generated-file marker and the directive form in the rule beside it are Go-specific. The syntax table already spans the C family and the hash family. A reader wanting a comment rule for TypeScript had no reason to look in a package named for Go.

## What it reads

The C family:

```
.go .c .h .cc .cpp .hpp .rs .java .js .mjs .cjs .ts .tsx .jsx .swift .kt .scala .zig
```

The hash family, plus a Makefile, a Dockerfile and a Justfile by name:

```
.py .rb .sh .bash .zsh .yml .yaml .toml .tf .just .mk
```

Go adds the raw literal, where a backslash escapes nothing.

## How it reads

It walks the bytes rather than parsing the language. A parser answers a question no rule here asks. Every parser worth using for the job also wants cgo, which the org's fat-APE builds cannot take.

What the walk must get right is narrow. It must never read a comment marker sitting inside a string, and never read a string delimiter sitting inside a comment. An unterminated literal consumes the rest of the file. A stray quote therefore cannot turn the remainder into prose.

## What it does not read

A file whose extension is absent from the table yields nothing at all. Guessing at a syntax reports a string literal as prose, and a verdict built on that is the kind a user turns off.

## The rules that sit on it

`commentnumbers` is the only rule reading this adapter today. The `tombstones` package carries a comment scanner of its own.
