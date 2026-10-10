# edit: the only way a repair writes

A repair never returns a rewritten copy of a file. It returns `edit.Edit` values, byte ranges with replacement text. The parser that owns the file then writes them through a gate. A gate checks each edit before it lands. It parses the result again and keeps an edit only when the tree still holds. A batch that fails is retried an edit at a time. A refused edit is reported in `Repair.Refused`.

| Gate | An edit may touch | After the splice |
|---|---|---|
| `treecomments.Apply` | bytes inside a comment node, and the blank around it | every node that is not a comment keeps its type, its text and its place in the tree. Every directive line comes back byte for byte |
| `markdown.Apply` | bytes inside a single CommonMark prose block | every verbatim block comes back as written, in order |
| workflow YAML | whole lines | the file parses, and a comment edit decodes to the same data |
| `goformat.Gate` | blank bytes, and it writes only blanks | the Go scanner reads every token as it was. A comment may lose the blanks that end its lines |

So a rewrite cannot escape its comment. A newline can end a line comment early. A closer can end a block early. An opener can swallow the code below. Each changes the code tree, and the gate refuses it. The interpreter line and a cgo preamble are code to the gate, because a tool reads them.

Every repair is a `fixer.Fixer`, and each package registers its fixers from `init` with `fixer.Register`. A fixer gets a `fixer.File` and changes it only through `File.Apply` or `File.ApplyComments`. The file has no text setter. The gates are its only writers. `slopfix.Fix` opens the file for its kind and runs `fixer.For(kind)` in `Order`. `fixers_test.go` pins that order. It fails on a repairable rule no registered fixer serves.

| Kind | Fixers, in order |
|---|---|
| source | `tombstones`, `comments/length`, `comments/number`, `comments/sentence-length`, `comments/length-after-number`, `pins/download-version`, `gofmt` |
| document | `tombstones`, `counts/inventory-count`, `counts/section-number`, `wrap-and-ste`, `wrap/long-block`, `ste/count`, `pins/download-version` |
| workflow | `yaml/ungate`, `yaml/join-comments`, `yaml/rename-guarded-job`, `comments/sentence-length`, `yaml/inline-env`, `yaml/filter-push`, `yaml/retarget-org-action`, `yaml/set-concurrency`, `pins/download-version` |

`gofmt` runs on a `.go` file that a fixer before it changed. A cut comment can leave a blank line too many, or bring together fields that gofmt aligns. The pass writes the gofmt layout through `goformat.Gate`. It answers to the selection of the fixers before it. A fragment with no package clause keeps its layout. So does a file no fixer changed, and a file whose gofmt layout changes more than whitespace.

The repository rules sit outside the registry. They delete or move whole files, and they edit no text inside one.

`edit.Scope` bounds where an edit may land, and follows the text through each pass. The hook passes the span its edit writes. `edit.Nowhere()` admits nothing, for a run that wants findings alone.

The string match left is the hook replaying an Edit payload. `old_string` is a literal by the tool's own contract, so the hook finds it the way the tool will.
