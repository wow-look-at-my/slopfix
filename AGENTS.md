# slopfix: the agent reference

slopfix is one binary for the org's prose rules. It also judges code comments, GitHub Actions workflows and the markdown files a repository keeps. It answers every Claude Code hook event that the marketplace plugin sends it. A hook, a CI job and an editor integration all call this binary. Each gets the same verdict.

`README.md` is for a person, `AGENTS.md` (this file) for an agent, and `CLAUDE.md` is the line `@AGENTS.md`. Depth that only one subsystem needs goes in `docs/<topic>.md`, with a pointer here.

## Build and test

```sh
go-toolchain            # builds build/slopfix, and runs the tests
GO_TOOLCHAIN_DATS_BUILD_DIR="$PWD/build" dats dats/no-work-loss.dats
```

`dats/` holds the CLI suites that go-toolchain runs after the build. They exec the built binary under a sandbox. A manual run therefore needs bubblewrap. A test that drives a foreign git hook belongs there.

`hooks.go` maps each hook to the rule IDs it runs. `hooks_test.go` fails the build on a rule with no hook entry. It also fails on an entry for a rule that does not exist.

## Commands

[docs/commands.md](docs/commands.md) holds this section.

## Rule table

| Category | Rule IDs | Repairs |
|---|---|---|
| `repo` | `repo/agents-file`, `repo/budget`, `repo/package-scripts`, `repo/binary` | yes, on a walk, except a `package.json` that does not parse |
| `repo`, report only | `repo/near-duplicate`, `repo/json`, `repo/xml` | no |
| `wrap` | `wrap/hard-wrap`, `wrap/long-block` | yes |
| `ste` | `ste/contraction`, `ste/modal`, `ste/semicolon`, `ste/comma-splice`, `ste/sentence-length`, `ste/postdeterminer`, `ste/count` | yes |
| `english` | `english/comma-never` | yes. `, never` becomes `, not`. Edited English rarely writes the first and often the second |
| `ste`, warnings | `ste/instruction-length`, `ste/passive`, `ste/noun-cluster`, `ste/tense`, `ste/dictionary`, `ste/paragraph-length` | no |
| `counts` | `counts/inventory-count` | yes |
| `tombstones` | `tombstones/date`, `tombstones/change-reference`, `tombstones/then-and-now-contrast`, `tombstones/position-reference`, `tombstones/hedged-time`, `tombstones/unstated-value`, `tombstones/shrug`, `tombstones/unexplained-workaround`, `tombstones/name-nothing-in-the-repository-defines`, `tombstones/comment-volume` | yes |
| `comments` | `comments/number`, `comments/length`, `comments/tail` | yes |
| `yaml` | `yaml/comment-block`, `yaml/all-builds-job`, `yaml/test-in-workflow`, `yaml/neutered-gate`, `yaml/env-indirection`, `yaml/push-tags`, `yaml/org-action-ref`, `yaml/concurrency` | yes |
| `pins` | `pins/download-version` | yes |
| message | `laziness/punt`, `blame/deflection`, `ask/prose-decision` | no |

`hooks.go` also lists `link-all-refs` as pending. Its detection lives in the `link-refs` guard, not in a rule ID.

Every error rule has a repair, even a crude one, so `slopfix fix` on any tree leaves no error. The exceptions are a `package.json` that does not parse and the `ReportOnly` rules, because no rewrite knows what the author meant. `repairable_test.go` names each of them. `allfix_test.go` runs `fix` over a tree of each rule's hardest case and requires a clean `check` after it. `repairable_test.go` fails on an error rule with no fixer and no repository pass behind it. `Fix` runs the fixers again until the text holds, because one repair can hand a later rule new text.

## CI action

The action at the repository root downloads the published binary from buildhost and runs `check .` with every rule. The step therefore goes after the checkout.

```yml
- uses: actions/checkout@v4
- uses: wow-look-at-my/slopfix@master
```

No input narrows the check. A path list, a rule list or a raw command line lets a caller set the gate to nothing. `action_test.go` fails the build on any input but `permission` and `level`. The action never repairs. A job that repairs its own checkout and then passes has enforced nothing.

`permission` runs `check workflow-permission --json` for the running job instead. `level` defaults to `write`. The `stdout` output holds the JSON answer. The values are quoted, so a value cannot add a flag.

```yml
- uses: wow-look-at-my/slopfix@master
  id: grant
  with:
    permission: id-token
```

On Unix the action runs the APE binary through `sh`, because a `binfmt_misc` handler can refuse a direct exec.

In a fork, only the lines the fork wrote can fail the check. The org's `.github` repository lists, in `fork-list/fork-of.json`, each fork GitHub does not record, and publishes it to buildhost. `docs/fork-scope.md` holds the detail.

## The marketplace follows each publish

cc-marketplace ships this binary inside its `slopfix` plugin. A publish here therefore reaches nobody until that plugin is packaged again. The last step of `ci.yml` does that on each master build. It dispatches `release.yml` in cc-marketplace with `publish: true`. The token is `CC_MARKETPLACE_DISPATCH_TOKEN` from secret-server, with `actions: write` on cc-marketplace. A missing token fails the build, because a silent skip leaves the marketplace on an old binary.

## repo: what a repository keeps

These rules judge the tree. Only a walk whose root holds `.git` reaches them. `check` reports them. `fix` applies them.

- `repo/agents-file`: a root `CLAUDE.md` that holds more than the `@AGENTS.md` import. `fix` moves its body into `AGENTS.md` and leaves `CLAUDE.md` as `@AGENTS.md` and a newline. Claude Code reads `CLAUDE.md`. Every other agent reads `AGENTS.md`.
- `repo/budget`: a root `README.md` or `AGENTS.md`, a `CLAUDE.md` anywhere, or a `.md` in a `claude_snippets/` directory, over `40000` characters. The repair writes `docs/` beside the file. The count is characters, because a byte count inflates a file with an em dash. `fix` moves the largest `##` sections into `docs/<heading>.md` until the file is at `32000` or less. The gap leaves room for the next edit. The text moves word for word, and each heading under it rises one level. The heading stays, with a link to the new file. A name that exists gets a `-2` suffix. With no `##` section left to move, the sections of the other heading levels move. A file with no heading at all moves its tail into `docs/<name>-continued.md` and keeps a link. A cut inside a fence closes the fence. The moved part opens it again.
- `repo/package-scripts`: a `package.json` with a `scripts` key. A `justfile` holds the commands instead. `fix` writes each script as a recipe in a `justfile` beside the manifest, and deletes the key. The recipe runs the command as written, with `node_modules/.bin` first on `PATH`. A `pre` or `post` script runs around its own, and `npm run x` becomes `just x`. A `package.json` that does not parse is also a finding, because no rule can read it. That finding has no repair.

- `repo/binary`: a file git tracks that opens with an ELF, Mach-O or PE/COFF magic number. `fix` deletes it, because a build makes it from source. Git still holds it. A tree that git cannot list is read from disk. The rule reads the blob git stores. A Git LFS file is never reported, because git holds its pointer and only the checkout holds the binary.
- `repo/near-duplicate`: a file whose lines match another file of its base name at `NearDuplicateShare` or above. The score is the Dice coefficient over non-blank trimmed lines, so a copy that differs in comments alone still trips. The later path of the pair is reported. A file each directory needs, such as `package.json`, `ts0.json`, a `justfile` or `Dockerfile`, is never compared. No attribute or list exempts a copy. The answer is one file that both places use. A symlink is that one file. As a result, it is never compared.
- `repo/json`: a `.json` or `.jsonc` file that does not parse, comments allowed, or that breaks the schema its `$schema` names. `wow-look-at-my/json-validator` does the check, the same library webhook-runner loads manifests with. A relative `$schema` is a path from the file.
- `repo/xml`: an `.xml` file that `wow-look-at-my/xml-validator` refuses, that names no schema, or that breaks the schema it names. The hint is read from the parsed root. No pattern over the raw text reads it. A root with no namespace names its schema in `xsi:noNamespaceSchemaLocation`. A root in a namespace names it in the `xsi:schemaLocation` pair for that namespace.

Both rules fetch a remote schema, once for each walk, and an XSD's imports with it. A schema that does not load, by a network error or any status but OK, is a finding. It is never a pass. The JSON Schema meta-schemas need no fetch, because the validator carries them.

The document rules walk what the other repository rules walk, so `testdata`, `node_modules`, a submodule and a nested clone stay out. `repo/binary` reads every tracked file, the large ones included.

A body that `AGENTS.md` already holds is not appended again. The import line is never copied into the file it imports. A `CLAUDE.md` that is a symlink stays.

Every other markdown file is left alone. The budget covers only the files that every request loads.

## edit: the only way a repair writes

A repair never returns a rewritten copy of a file. It returns `edit.Edit` values, byte ranges with replacement text. The parser that owns the file then writes them through a gate. A gate checks each edit before it lands. It parses the result again and keeps an edit only when the tree still holds. A batch that fails is retried an edit at a time. A refused edit is reported in `Repair.Refused`.

| Gate | An edit may touch | After the splice |
|---|---|---|
| `treecomments.Apply` | bytes inside a comment node, and the blank around it | every node that is not a comment keeps its type, its text and its place in the tree. Every directive line comes back byte for byte |
| `markdown.Apply` | bytes inside a single CommonMark prose block | every verbatim block comes back as written, in order |
| workflow YAML | whole rows | the file parses, and a comment edit decodes to the same data |
| `goformat.Gate` | blank bytes, and it writes only blanks | the Go scanner reads every token as it was. A comment may lose the blanks that end its lines |

So a rewrite cannot escape its comment. A newline can end a line comment early. A closer can end a block early. An opener can swallow the code below. Each changes the code tree, and the gate refuses it. The interpreter line and a cgo preamble are code to the gate, because a tool reads them.

Every repair is a `fixer.Fixer`, and each package registers its fixers from `init` with `fixer.Register`. A fixer gets a `fixer.File` and changes it only through `File.Apply` or `File.ApplyComments`. The file has no text setter. The gates are its only writers. `slopfix.Fix` opens the file for its kind and runs `fixer.For(kind)` in `Order`. `fixers_test.go` pins that order. It fails on a repairable rule no registered fixer serves.

| Kind | Fixers, in order |
|---|---|
| source | `tombstones`, `comments/length`, `comments/number`, `comments/length-after-number`, `pins/download-version`, `gofmt` |
| document | `tombstones`, `counts/inventory-count`, `wrap-and-ste`, `ste/count`, `pins/download-version` |
| workflow | `yaml/ungate`, `yaml/join-comments`, `yaml/rename-guarded-job`, `yaml/untest`, `yaml/inline-env`, `yaml/filter-push`, `yaml/retarget-org-action`, `yaml/set-concurrency`, `pins/download-version` |

`gofmt` runs on a `.go` file that a fixer before it changed. A cut comment can leave a blank line too many, or bring together fields that gofmt aligns. The pass writes the gofmt layout through `goformat.Gate`. It answers to the selection of the fixers before it. A fragment with no package clause keeps its layout. So does a file no fixer changed, and a file whose gofmt layout changes more than whitespace.

The repository rules sit outside the registry. They delete or move whole files, and they edit no text inside one.

`edit.Scope` bounds where an edit may land, and follows the text through each pass. The hook passes the span its edit writes. `edit.Nowhere()` admits nothing, for a run that wants findings alone.

The string match left is the hook replaying an Edit payload. `old_string` is a literal by the tool's own contract, so the hook finds it the way the tool will.

## markdown: the document model

`Split` breaks a document into prose blocks and verbatim blocks with a CommonMark parser. A fence, a table, a heading and a blank run are verbatim. No rule reads or reflows them. Every prose rule gets its fence, code and list exemptions from this split. A block keeps its list marker and indentation. A nested item thus survives a rewrite.

`wrap/hard-wrap` rejects a paragraph split over several source lines. It reports each continuation line. The reader's window wraps a paragraph. An author's wrap freezes one window's width into the file. Each later edit then re-flows untouched lines.

`fix` writes each prose block back as a single line. `WordsOnly` proves the join moved only newlines. A rewrite whose words differ from the source is refused. The caller keeps the original. A workflow is never joined, because a newline in YAML is syntax.

`wrap/long-block` rejects a paragraph or a list item over `LongBlockCap` characters, which is `1500`. A list item counts too. `fix` divides it into paragraphs of `LongBlockTarget` characters and `LongBlockWordTarget` words or less, with a blank line between them. The word bound keeps each part under a paragraph cap that counts words. A list item indents each later part to its content. As a result, the part stays in the item. A division lands at a sentence end, then at a sentence end inside a parenthesis, then between words. It never lands inside a code span, a link, bold text or a quotation, or before text that opens a block. The markdown gate admits the blank line, and refuses an edit that moves a paragraph to another container.

## ste: Simplified Technical English

[docs/ste-simplified-technical-english.md](docs/ste-simplified-technical-english.md) holds this section.

## syntax: the sentence parser

`syntax` holds no rule ID. It parses an English sentence into phrases and clauses, where a regular expression cannot tell the grammar apart. The `prose` tokenizer keeps each word's byte span. Its averaged-perceptron tagger assigns Penn Treebank tags, as in Vale. A retag pass restores verbs the tagger reads as nouns. Chunking builds noun phrases, each with its determiner, numerals and head. The clause pass cuts at conjunctions, subordinators, relative words and punctuation. It gives each clause a subject, a finite verb and a depth.

A word inside an opaque span reads as a name. Code spans, parentheticals and quotations pass this way, because their words are data. The word classes live in `rules/syntax.xml`. `github.com/jdkato/prose/v3` is MIT.

```
[The gate] <reads> [every file] and <refuses> [the write] .
opens/0 subject="The gate" verb="reads": The gate reads every file
coordinate/0 subject="-" verb="refuses": and refuses the write
```

## cardinal: is this number a count

`cardinal` holds no rule ID. It decides whether a number is a stated count, true today and wrong after the next commit. Each caller brings its substrate. The substrate is the policy.

| | `Prose` | `Gate` | `Comment` |
| --- | --- | --- | --- |
| Rule | `counts/inventory-count` | `ste/count` | `comments/number` |
| Shape | quantity | quantity | number |
| Frame required | yes | no | no |
| Vocabulary | two upward, plus a dozen | two to twelve | cardinals, ordinals, scales, repeat counts |
| Exemptions | function-word gap, a longer number, arithmetic, status code, label, unit before the noun | arithmetic, function-word gap, status code, label, unit before the noun | status code, exit status, literal, section sign, currency, quotation |

Prose requires a frame, because a document carries numbers that count nothing: a version, a port, an example. The gate needs no frame. That is the whole difference between the document substrates. A number beside code is nearly always a count. A comment therefore needs no frame either. The vocabularies stay separate, because widening one changes the verdict on text nobody edited. `Find` returns the whole quantity for prose. That `cardinal.Leading` can cut its number. For a comment it returns the number alone.

## counts: a count in a document

`counts/inventory-count` cuts the cardinal out of a sentence that counts what is here. `there are three sections` becomes `there are sections`, which stays true. The org rules that a count in markdown is not worth maintaining.

Every reported count gets a repair. Where a bare cut breaks the sentence, `counts/reword.go` writes words that state no figure. A rate becomes `every few`. A cap becomes `a bounded number of`. A unit takes `a couple of`, `a few`, `several` or `many`, by size. After a preposition or a noun the number becomes `multiple`. A hedge such as `about` or `exactly` goes with the number. A bare number that the same clause sets beside it, as in `warns at 500 lines and errors at 750`, says how it compares: `a higher count`, `a lower count` or `the same count`. No clause stays half converted.

A count needs a frame and a quantity on the same line. The quantity is a cardinal that governs a plural noun. The frame is a possessive (`this repo's plugins`), a having verb (`it ships hooks`) or a deictic (`the rules below`). A quantity with no frame is ordinary technical prose.

It does not flag the singular, which is overwhelmingly a pronoun in English. That gap is stated rather than closed. It drops a quantity reached through a function word, as in `2 of the format drops`. It drops a match that continues a longer number, such as a version. Fences, tables, headings and backtick spans never reach it. A measurement inside a frame IS reported, because the number belongs where it is enforced.

## tombstones: a comment about a state the code has left

A tombstone describes a state the code has left, or argues for the diff instead of telling the next editor what breaks. The wording tier is the weakest. A paraphrasing machine writes the judged text. It eventually writes around a phrase rule.

- The wording tells are `pattern` entries with an `id` in `english/english.xml`. Each cuts the PHRASE, not the line. The rest of the sentence thus survives. A pattern with no `id` still rewrites.
- `tombstones/name-nothing-in-the-repository-defines` reads no wording. A comment that names a symbol found nowhere in the repository describes a tree that is gone. The probe runs ripgrep on the working tree. A tree walk, and any run past `50` files, first reads the identifiers of every file git lists. That list holds tracked files and untracked files git does not ignore. The index then answers every name with no probe.
- A pattern cut never removes a negation while the words it negates stay. `no longer` is therefore a flag, not a cut.
- A copyright year is cut on purpose. A yearly bump only pads a commit.
- `tombstones/comment-volume` counts the lines of a merged comment run against the cap, which defaults to `14` lines. No rewording defeats it. The repair cuts the run from its end down to the cap, the way `comments/length` cuts. With no sentence left to cut, it drops whole lines and closes the last sentence it keeps.
- A dead name that no whole-line strip resolves loses the sentence that holds it. On a line shared with code, the cut stops at the comment marker.

A referent candidate must look like a symbol: an underscore, an internal case change, or a capital beside a digit. A name in capitals and a short name never qualify. The probe answers nothing when it cannot answer: no repository, no ripgrep, a timeout or an error. An absent answer must never read as a missing symbol. A comment that names too many candidates is skipped.

Only prose is judged. In source that means the comments. A string that holds `previously` is thus safe. In a document it means the paragraphs, without fences, HTML comments, frontmatter or backtick spans. A file with no known comment syntax is not judged. A document has no volume cap.

A referent line strips only when the comment is alone on its source line. A document finding never strips, because a document line is a paragraph. The block is rewrapped after a strip. The response names each deleted line. Git history keeps the rest.

`--path` is required on stdin, because the path decides the comment syntax.

```sh
slopfix fix --only tombstones --max-comment-lines 0 --path pkg/thing.go < pkg/thing.go
```

## comments: what a comment gets wrong

The `commentfix` package reads a real syntax tree through `go-tree-sitter`. A comment is a node whose type carries `comment`. The documented construct is the next named sibling. No rule names a language. A file that does not parse yields nothing, because half a tree is worse than none.

The languages are Go, C, C++, Rust, Bash, JavaScript, TypeScript and TSX. YAML. TOML, `.conf` and `.zsh` files read with the Bash grammar. `languages_test.go` refuses a grammar that no fixture proves. `selfrepair_test.go` runs the rules over this tree. The go-toolchain vet phase runs them on every build.

- `comments/number`: a number in a comment, read with the `Comment` substrate. The repair says it in words. A count of a plural noun that no table entry covers gets the `counts` rewording. Only a sentence that still holds a number after that is cut. To point at a section, cite its slug or heading, not its position.
- `comments/length`: a comment run weighed against the construct beneath it. Lines catch an essay. Characters catch a dense paragraph. The budget has a floor. A short comment is never a finding.
- `comments/tail`: a comment that stops on a word that opens what a cut took away. The repair closes the sentence.

The length repair cuts from the end, because a comment leads with its point. Each cut lands on a sentence end. The opening sentence is never cut mid-clause. When no cut fits, the opening sentence stays, repaired to STE. One over the 25-word cap closes at a clause boundary the `syntax` parser finds, the same boundaries `ste/sentence-length` divides at (`ste.Leading`). Sentence ends come from `ste.Sentences`. When no clause boundary fits either, `wordFit` keeps the longest run of leading words that fits. The run never ends on a dangling word or inside a parenthesis, quotation or code span, and it closes as a sentence. Every finding therefore has a repair. The number repair runs after the length cut. The length cut then runs again if the words overflow.

It does not flag a comment above the package declaration, or a trailing comment on a code line. It skips a directive line, such as a build constraint, a shebang, a linter pragma or a cgo preamble. It skips a number inside a quotation.

## yaml: workflow and action manifest rules

These rules judge the format. The prose rules never run on these files. A file is read when its base name is `action.yml` or `action.yaml`, or when it is YAML under a `workflows` directory. Other YAML with a left-margin `jobs:` or `runs:` key qualifies too.

- `yaml/comment-block`: a run of comment lines past the single-line limit. A blank line neither counts nor ends a run. The repair joins the run into the allowed line and keeps every word.
- `yaml/all-builds-job`: a job named `all-builds`, by key or rendered name. The required gate is a commit status from the required-builds-manager app. A job with that name shadows it. The repair renames the job to `builds` and fixes each `needs` entry.
- `yaml/test-in-workflow`: a test inside a `run:` script. That is an assertion with a nonzero exit, a function whose name says it asserts, or a redirect to a test file. The repair deletes the assertion lines, because a test belongs in the suite.
- `yaml/neutered-gate`: a gate step under `continue-on-error`. A step allowed to fail is not a gate. The repair deletes that line, found by parser positions.
- `yaml/env-indirection`: a step `env:` entry with no job to do. Its value is a single `${{ }}` expression that the script reads only as `$NAME` or `${NAME}`. Or the script sets the variable before it reads it. A runner variable such as `$RUNNER_TEMP` is also reported, because a context names it. A container job and a composite action keep the variable, because the context names the host path there. The repair writes the expression into the script, deletes the entry, and deletes an `env:` key with nothing left under it.
- `yaml/push-tags`: a `push` trigger with no `branches`, `branches-ignore`, `tags` or `tags-ignore` filter. Every tag push then starts the workflow again. The repair writes `branches: ['**']` under `push:`. A shape no row edit reaches, such as `push: {}`, gets its whole `on:` value written again in block style.
- `yaml/org-action-ref`: a `uses:` of a `wow-look-at-my` action or reusable workflow at a ref other than `master`. A `wow-look-at-my/actions` orphan tag `<directory>#latest` is published from master. As a result, it passes. A feature branch breaks the moment it merges, and an agent that pins one to dodge a check has bypassed the org's tooling. The repair writes `@master`. A numbered orphan tag becomes `#latest`, and a bare `actions@<directory>` becomes `<directory>#latest`.
- `yaml/concurrency`: a workflow whose top-level `concurrency:` is not the org's block. Off master the group names the repository, the workflow and the ref, and `cancel-in-progress` is true. A push then cancels the run before it. On master the group holds the run ID, so every run completes. The workflow name is in the group because a group spans every workflow in the repository. The repair writes the block after `on:`, or writes the old block again. A reusable workflow is skipped, because it runs in the caller's context, and the same group then deadlocks. A job-level `concurrency:` stays the author's.

The all-builds wording is the operator's own. It must not be softened. A job that wears the required status's name is a known deception attempt.

Unparseable YAML yields no all-builds finding, because the runner fails on it anyway. A job name that holds an expression is skipped. A matrix suffix and a reusable workflow's path parts are stripped. A segment must then match exactly. A step that only runs a command is not a test. An error annotation alone is a report. Every repair works on whole lines and never reflows.

## pins: a download URL that names a release

`pins/download-version` rejects a `dl.pazer.build` URL with a `v` query parameter. It reads every file `check` reads, code strings included, except a test file (`*_test.*`, `*.test.*`, `*.spec.*`, `test_*.py`, `__tests__/`). A test asserts the exact URL the code under test produces. A URL with no `v` serves the newest published build on the default branch. A pinned one breaks when that release is gone.

`net/url` reads the query. The repair deletes `v` and writes the query back with `Encode`. `pins.Gate` admits only that rewrite, because the source gate lets an edit touch comments alone. `Encode` escapes `${OS}` and writes `&` for `&amp;`. A URL that holds either loses `v` as text instead, and keeps every other byte. A `${{ }}` expression is part of the URL, blanks included.

## laziness: a turn that ends with the work undone

`laziness/punt` reports a closing message that leaves the work undone because the work is harder. It covers a found defect left in place, and permission asked in place of action. Both leave the reader holding the work. One table therefore holds both.

The shapes are: disowning a defect you found, handing a repair back, leaving a defect in place. Excusing yourself from a repair, announcing an attribution hunt, offering authorship in place of a fix. And asking permission in place of acting.

```
$ printf 'Want me to fix it?\n' | slopfix check --message --only laziness --json
{"path":"","findings":[{"id":"laziness/punt","line":1,"endLine":1,"rule":"asking permission in place of acting","detail":"Want me to fix it?","fix":"Do the work the sentence hands back, then say what you did.","repairable":false,"severity":"error"}]}
```

A sentence that carries a shape AND says the repair happened is not a punt. A fix claimed in another sentence pardons nothing. A bare diagnosis is legitimate. The word `pre-existing` is thus absent from the table. What fires is the diagnosis given as the reason to stop. An honest deferral that names a real blocker is clean. Fences, indented code and blockquotes are exempt. Inline backticks are not. A sentence reports a single shape.

It reports only, because the repair is work. The `laziness` hook guard runs it at Stop, because only a Stop hook can stop the model stopping. The hook answers with the single word the reader types back. It gives nothing to argue with. The guard reads the transcript when the payload carries no message.

## blamelanguage: a message that deflects blame

`blame/deflection` marks a message that deflects the work onto another author or an earlier time. It never refuses. A Stop refusal runs after the message streams. It unsends nothing. The retype then loops without a bound. The standing rule for a guard here is to mitigate rather than refuse.

The `blame-language` hook guard runs it on MessageDisplay and appends one line for the reader. Nothing goes to the model. `displayContent` changes the screen and not the stored message. `CC_NO_BLAME_LANGUAGE=0` disables it. Every failure prints nothing, because a guard that eats output is worse than none.

The message is judged whole. A per-message state file keyed by `message_id` accumulates the flushes. The final flush drops it. A lost file costs earlier phrases, not a wrong mark.

`bannedPhrases` is a plain slice. It holds provenance openers such as `that predates this session` and `git blame shows`. It also holds `pre-existing` and narrow synonyms such as `not related to my change`. Matching is case-insensitive over whitespace-normalized text, with source offsets kept. Fences, indented code and blockquotes are exempt. Inline backticks are not. A real blocker named plainly is never marked.

## ask-properly, link-refs, busy-poll, md-budget

These are hook guards. `check --message` also runs `ask/prose-decision`, and `repo/budget` covers the files `md-budget` measures.

- `ask-properly` appends a line when a message puts a decision to the reader in prose. `AskUserQuestion` renders the choices instead. It refuses nothing and judges the message whole.
- `link-refs` renders a pull request, a commit or a branch as a markdown link as the message streams. A target it cannot show to exist stays plain text. A state file carries the fence state across flushes.
- `busy-poll` serves Stop and PreToolUse. On Stop it refuses a turn that repeats the same call seconds apart. On PreToolUse it refuses a status read of a subject this session saw settle. It runs in a remote session only, because waiting for an event needs events. Evidence ages out. Every failure allows.
- `md-budget` keeps each `CLAUDE.md` and each `claude_snippets/` file under `40000` characters, because every request re-sends them. It reports at session start and on a write. It blocks a Stop that leaves one broken. `CC_CLAUDE_MD_BUDGET` sets the budget. `CC_CLAUDE_MD_WIDTH` turns on the wrap check.

`english/english.xml` also links an `owner/repo#N` reference in a message. That pattern cannot check existence. The `link-refs` guard thus stays the verified path.

## auto-allow

`auto-allow` approves read-only work and refuses a program this environment does not run. The table is `autoallow/rules.xml`, embedded and checked by `rules.xsd`. MCP trust is per SERVER, not per tool-name pattern.

The binary is registered on PermissionRequest and PreToolUse. In `cli.js` (checked at `2.1.220`), `createCanUseTool` evaluates the rules first. PermissionRequest hooks run only on the "ask" path. `defaultMode: "auto"` answers first. A deny that rides PermissionRequest thus never fires in auto mode.

PreToolUse fires before the permission pipeline, whatever its outcome. A deny there stops the tool. The reason reaches the model as an error result. Deny therefore rides PreToolUse. Allow rides PermissionRequest, where it never outranks the user. `denyOnly` in `run.go` drops every non-deny verdict on PreToolUse. The events want different output objects. `decisionPayload` therefore branches on the event. The verdict comes from `tool_input` alone. A call judged on both events is therefore harmless.

`rules.xml` has no `<mcpServer name="Claude_Code_Remote">` block, on purpose. Do not add one. The CCR proxy answers a call with `-32003` ("needs_approval"). The client then shows an approval card and runs the hooks beside it. A hook allow calls `cancelRequest`, which kills the card and the listener that a human click resolves through. The single retry carries no grant. The model then sees `MCP error -32003: MCP tool call requires approval`.

Nothing else automates the click. The retry hands `canUseTool` a prebuilt `ask` decision. No rule, mode or classifier runs. `extraAllowedTools` does not exist in the bundle. `serverApprovalWatch` is the shape of a real upstream fix. `anthropics/claude-code#81362` tracks it. Without `send_later`, use the `/loop` skill. Watch a pull request with `mcp__github__subscribe_pr_activity`.

## no-work-loss

`no-work-loss` asks questions of one parsed command. Destruction: does it destroy content that exists only in the working tree? Provenance: does it change file content without Write, Edit or NotebookEdit?

`docs/no-work-loss.md` holds the detail: what each verb destroys, how a command is parsed and wrapped, and which write shapes are refused.

## clean-bash

`clean-bash` rewrites a Bash command rather than refusing it, wherever a rewrite exists. The rules run on the parse tree to a fixed point. An unparseable command passes through. A rewrite is silent: it emits only the new input and `suppressOutput: true`. A visible message gives the model something to blame. A denial carries a reason. Without one, the model retries forever. `CLEANUP_BASH_CMDS_LOG=/path` logs each rewrite and each deny.

Denials: `heredoc`, `perl` (`^perl[0-9.]*$` as the effective command), `file_read` (`cat`, `head`, `tail` or a line-selecting `sed` on a file), `shred`, `git_rm` with `--pathspec-from-file`, `truncate_zero` with an unknown flag, `rm_flag` for an `rm` flag it cannot drop. And `toolchain_capture` for `go-toolchain` inside a substitution.

A command hook cannot swap Bash for Read. A read that `readplan.go` maps onto Read is therefore not denied: it runs. The `read-output` guard then rewrites its output on PostToolUse as the Read tool prints those lines, with a note that names the Read calls. `readplan.go` maps one plain call with no pipe or redirect: `cat`, `head -n N`, `tail -n N`, `tail -n +K`, or `sed -n 'A,Bp'`. The path is made absolute against the payload `cwd`. A `tail` count from the end reads the line total of the file. A read it does not map keeps the `file_read` deny. `check read-plan` prints the mapping for one command.

- `devnull` removes `2>/dev/null` in any spelling. It matches a parsed redirect. A `12>/dev/null` thus stays.
- `rm_recycle`, `truncate_recycle` and `find_delete_recycle` turn deletion into `recycler trash`. Delete-then-Write is the loophole around the Write refusal.
- `git_rm_recycle` writes `git rm` as `git rm --cached` and then `recycler trash` on the same paths, inside each `-C` directory. A dry run passes through.
- `head_tail`, `grep`, `or_true` and `stderr_merge` drop a trailing `| head`, `| tail`, `| grep`, `|| true` or `2>&1` from the final statement.
- `tee` turns a trailing stdout redirect on the final statement into `| tee file`.
- `toolchain_output` strips every pipe stage or stdout redirect from `go-toolchain`.
- `docker_compose_restart` writes `docker compose up -d --force-recreate`.
- `grep_json` turns a `grep`, `egrep`, `fgrep` or `rg` over JSON files into `jq`. A fixed head and tail of each file decide, not the extension. JSON Lines prints each matching record. A document prints `.path = value` for each matching leaf. An unknown flag or a non-JSON operand leaves the grep alone. `bashclean/grepjson.xml` holds the programs, their flags, the fixtures and the tests beside each entry.
- `gh_wait_ci` maps `gh run view`, `watch`, `rerun`, `list` and `gh pr checks` to `gh wait-ci`.
- `sleep_cap` writes `sleep 3` for any sleep past `3` seconds or not literal.
- `narration_remove` turns an `echo` that reaches the terminal into `:`.
- `pipefail` injects `set -o pipefail`. A `tee` then never masks a failure.

## What no rule reads

A fenced code block, a table and a heading are data. No rule reads them and no repair reflows them. An HTML entity and an inline code span are masked before a rule sees the text.
