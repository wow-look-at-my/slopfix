# slopfmt

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
| `slopfmt fix` | Read a document on stdin and write the repaired text back. |
| `slopfmt counts` | Read a document on stdin and cut the cardinal out of each inventory count. |
| `slopfmt tombstones` | Read added text on stdin and strip each tombstone comment out of it. |
| `slopfmt purge <paths>` | Delete the instruction files that no longer earn their place. |

Every stdin command takes `--json`, which writes the whole answer as one object. That is the shape a hook reads.

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
