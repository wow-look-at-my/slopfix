# slopfmt

<<<<<<< HEAD
slopfmt is one binary that holds this org's prose rules. It formats a document. It reports what fails the merge gate. It repairs what a machine can repair without guessing.

The rules live here once. CI runs it. An editor runs it. A Claude Code plugin shells out to it. None of the three can drift from the others, because none of them carries a copy of a rule.

## Install

```sh
curl -fL --compressed "https://dl.pazer.build/slopfmt?os=linux&arch=amd64" -o /usr/local/bin/slopfmt && chmod +x /usr/local/bin/slopfmt
```

The `os` parameter takes `linux`, `darwin` or `windows`. The `arch` parameter takes `amd64` or `arm64`.

## Commands

| Command | What it does |
|---|---|
| `slopfmt check <paths>` | Report every finding. A finding exits non-zero. |
| `slopfmt fmt <paths>` | Rewrite each file in place. |
| `slopfmt fix` | Read text on stdin and write the repaired text back. |
| `slopfmt purge <paths>` | Delete the instruction files that no longer earn their place. |

`fix` takes `--only` to run a single rule, and `--path` to name the file the text is headed for. The path decides the comment syntax, and the tombstone rule needs it.

```sh
slopfmt fix --only counts --json < notes.md
slopfmt fix --only tombstones --path loader.go --json < added.txt
```

`--json` writes the whole answer as one object. That is the shape a hook reads.

## The rules

**Sentence length.** A sentence over 25 words fails. The cap comes from ASD-STE100, which caps an instruction at 20 words and a description at 25.

**Contractions, and the modals this org bans.** Write `does not` rather than `doesn't`. Write `must` rather than `should`.

**Punctuation.** A semicolon and a comma splice each join two sentences that read better apart.

**Hard wraps.** A paragraph is one line. A wrap is one author's guess at one reader's window, frozen into the file. It turns a two-word change into a diff that reads as a rewrite.

**Inventory counts.** A number that counts what a repository holds is wrong the moment somebody adds one. `there are three sections` becomes `there are sections`, which stays true.

**Tombstones.** A comment carries current truth. It does not carry what the code used to be, when it changed, or who asked for the change. Git already holds all of that.

A fence, a table and a heading are verbatim. No rule reaches inside one.

## Repair, never refusal

A caller that already knows the answer must not spend a round trip asking for it. So `fix`, `counts` and `tombstones` each return the repaired text. A finding survives only when no deletion resolves it, and a caller refuses on those.

## License

MIT. See [LICENSE](LICENSE).
=======
One tool for the prose rules this org applies to a markdown file, and for the same rules a code comment must follow.

## Build

```sh
go-toolchain            # builds build/slopfmt, and runs the tests
```

## Use

```sh
slopfmt check docs/*.md   # report what the rules reject, and exit 1 when anything does
slopfmt fmt docs/*.md     # join every wrapped paragraph back to a single line
slopfmt purge .           # delete the markdown a repository must not keep
```

## What check reports

- A paragraph split over several lines. The reader's window wraps a paragraph. An author's wrap freezes one window's width into the file.
- A semicolon, a contraction, and a banned modal. STE approves none of them.
- A comma joining clauses that each stand alone. The rule wants a subject and a finite verb after the comma.
- A sentence over the word cap. Text in parentheses counts as a single word, which keeps a citation from inflating the count.
- A stated count of items. The number is true until somebody changes the set, and nothing corrects it then.

## What check never reads

A fenced code block, a table and a heading are data. No rule reads them. The `fmt` command never reflows them.

An HTML entity ends in a semicolon, and an inline code span holds whatever it holds. Both are masked before a rule sees the text.
>>>>>>> origin/master
