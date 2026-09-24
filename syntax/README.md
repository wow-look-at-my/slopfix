# syntax

This package parses an English sentence into phrases and clauses. The prose rules use the parse where a regular expression cannot tell the grammar apart. Examples are a numeral between a determiner and its noun, and the clause boundary where a long sentence divides.

## How it parses

1. `prose`'s tokenizer splits the sentence into words. Each word keeps its byte span in the source.
2. `prose`'s averaged-perceptron tagger gives each word a Penn Treebank tag.
3. A retag pass corrects the tags a clause test depends on. The tagger reads "the write fails" as a plural compound noun, so a stretch with no finite verb gets its verb back.
4. Chunking groups the words into noun phrases and verb groups. A noun phrase records its determiner, the numerals after it, and its head.
5. The clause pass cuts the sentence at conjunctions, subordinators, relative words and punctuation. It gives each clause a subject, a finite verb and a depth. A main clause has depth zero.

A word inside an opaque span reads as a name. The rules pass code spans, parentheticals and quotations this way, because their words are data.

The word classes the passes read live in [rules/syntax.xml](../rules/syntax.xml).

## Seeing a parse

```sh
slopfix parse "The gate reads every file and refuses the write."
slopfix parse --tags "The loader opens the file, which is slow."
```

`parse` prints the sentence with noun phrases in `[ ]` and verb groups in `< >`, then one line per clause:

```
[The gate] <reads> [every file] and <refuses> [the write] .
opens/0 subject="The gate" verb="reads": The gate reads every file
coordinate/0 subject="-" verb="refuses": and refuses the write
```

## Licenses

`github.com/jdkato/prose/v3` is MIT. Vale uses the same tagger.
