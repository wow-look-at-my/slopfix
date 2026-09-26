# slopfix: the agent reference

slopfix is one binary for the org's prose rules. It also judges code comments, GitHub Actions workflows and the markdown files a repository keeps. It carries the Claude Code hook subcommands that the marketplace plugins exec. A hook, a CI job and an editor integration all call this binary. Each gets the same verdict.

A repository keeps only `README.md` (for a person), `AGENTS.md` (this file) and `CLAUDE.md` (the line `@AGENTS.md`). Put depth here, not in a new markdown file. The `repo/stray-markdown` rule deletes any other markdown file.

## Build and test

```sh
go-toolchain            # builds build/slopfix, and runs the tests
GO_TOOLCHAIN_DATS_BUILD_DIR="$PWD/build" dats dats/no-work-loss.dats
```

`dats/` holds the CLI suites that go-toolchain runs after the build. They exec the built binary under a sandbox. A manual run therefore needs bubblewrap. A test that drives a foreign git hook belongs there.

A green master build starts `release.yml` in `wow-look-at-my/cc-marketplace`, because that plugin ships a copy of this binary. The token is `CC_MARKETPLACE_DISPATCH_TOKEN` from secret-server. The job fails when the token is absent.

`hooks.go` maps each hook to the rule IDs it runs. `hooks_test.go` fails the build on a rule with no hook entry. It also fails on an entry for a rule that does not exist.

## Commands

```sh
slopfix check [path...]           # report what the rules reject, exit 1 on any finding
slopfix check --fix [path...]     # repair in place first, then report what is left
slopfix fix [path...]             # the same as check --fix
slopfix check --path doc.md < doc.md   # judge stdin as text headed for doc.md
slopfix parse "The gate reads every file."
slopfix report --path doc.md < doc.md  # JSON findings, for the editor plugin
slopfix message < message.txt     # judge a closing message
```

- A directory argument is walked. The walk skips hidden directories except `.github`. It also skips `vendor`, `node_modules`, `testdata`, `build`, registered submodules and nested Go modules.
- A named file is read whatever its extension. The path decides the rules. A workflow or action manifest gets the `yaml` rules. A document gets the prose rules. Source gets the `comments` rules.
- A document is `.md`, `.markdown`, `.mdown` or `.txt`. An empty `--path` also counts as a document.
- With no path argument, `check` reads stdin. A repair goes to stdout. The findings go to stderr. `--json` writes the whole answer as one object, which is what a hook reads.
- `--max-comment-lines` sets the tombstone volume cap. `0` turns the cap off.
- `fmt`, `purge`, `comments` and `workflows` do not exist as commands. The wrap join is the `wrap/hard-wrap` rule. The purge is the `repo` category. The comment and workflow rules run inside `check` on each file they judge.

### Selecting rules with --only

`--only` takes a comma-separated list. An entry is a category, or a rule ID inside a category. A rule ID is the name the report prints beside a finding, the way a compiler names a warning.

```sh
slopfix fix --only counts            # every rule in the counts category
slopfix fix --only ste/semicolon     # that rule alone
slopfix fix --only tombstones,wrap   # two categories
slopfix fix --only ste/nosuch        # an error that names the rules ste holds
```

The categories are `tombstones`, `counts`, `wrap`, `ste`, `comments`, `yaml` and `repo`. A rule ID turns its category on. An unknown name is an error, because a run that applies nothing reads as a clean file. There is no `--exclude`. An exemption that a caller writes is one that a caller sets to everything.

A word repair and the wrap join share a pass. A rule reads a paragraph as a sentence stream. A hand wrap hides half of it. So an `ste` rule also joins the paragraph it repairs.

### Other commands

- `slopfix parse [--tags]` prints noun phrases in `[ ]` and verb groups in `< >`. It then prints each clause with its kind, depth, subject and verb.
- `slopfix report --path P [--only IDs]` reads a document on stdin and writes its findings as JSON. `--path` is required, because the path decides the rules. It always exits 0.
- `slopfix hook` reads a PreToolUse payload for Write, Edit or MultiEdit. It repairs the text the write adds and lets the write through. It flags what the repair did not reach.
- The hook repairs an Edit where it lands in the file. A line inside a fence therefore stays code. A `Scope` holds every rewrite inside the edit's own bytes, so a repair the file needs elsewhere never lands. The hook reports those findings instead.
- The hook prints nothing and exits 0 for anything it does not judge. That covers a bad payload, another event, a tool that writes no file, and clean text.
- `slopfix message [--only ID] [--json]` judges a closing message on stdin and exits 1 on a finding. `--only` also takes a family such as `blame`.
- The hook subcommands are `auto-allow`, `busy-poll`, `no-work-loss`, `md-budget`, `link-refs`, `clean-bash`, `ask-properly`, `laziness` and `blame-language`. Each reads a hook payload on stdin and writes the hook's response on stdout. A refusal is an exit code that the launcher passes through.
- `--trace` on any command prints a timing breakdown by phase.

## Rule table

| Category | Rule IDs | Repairs |
|---|---|---|
| `repo` | `repo/stray-markdown`, `repo/agents-file`, `repo/budget` | the first two, on a walk |
| `wrap` | `wrap/hard-wrap` | yes |
| `ste` | `ste/contraction`, `ste/modal`, `ste/semicolon`, `ste/comma-splice`, `ste/sentence-length`, `ste/postdeterminer`, `ste/count` | yes, except a long sentence with no clause boundary |
| `counts` | `counts/inventory-count` | yes |
| `tombstones` | `tombstones/date`, `tombstones/change-reference`, `tombstones/then-and-now-contrast`, `tombstones/position-reference`, `tombstones/hedged-time`, `tombstones/unstated-value`, `tombstones/shrug`, `tombstones/unexplained-workaround`, `tombstones/name-nothing-in-the-repository-defines`, `tombstones/comment-volume` | all but the volume cap |
| `comments` | `comments/number`, `comments/length`, `comments/tail` | yes, except a block no cut can fit |
| `yaml` | `yaml/comment-block`, `yaml/all-builds-job`, `yaml/test-in-workflow`, `yaml/neutered-gate` | yes |
| message | `laziness/punt`, `blame/deflection` | no |

`hooks.go` also lists `ask-properly` and `link-all-refs` as pending. Their detection lives in the `ask-properly` and `link-refs` subcommands, not in a rule ID.

## CI action

The action at the repository root downloads the published binary from buildhost and runs `check` on `paths`.

```yml
- uses: actions/checkout@v4
- uses: wow-look-at-my/slopfix@master
  with:
    paths: .
    only: yaml/comment-block
```

`paths` defaults to `.`. The step therefore goes after the checkout. `only` is the `--only` flag. The action has no `command` input. It never repairs. A job that repairs its own checkout and then passes has enforced nothing. On Unix the action runs the APE binary through `sh`, because a `binfmt_misc` handler can refuse a direct exec.

## repo: the markdown a repository keeps

These rules judge the tree. Only a walk whose root holds `.git` reaches them. `check` reports them. `fix` applies them.

- `repo/stray-markdown`: a `.md` file other than the root `README.md`, `AGENTS.md` and `CLAUDE.md`. A `README.md` in a subdirectory is stray too. `fix` deletes it. Move its content first.
- `repo/agents-file`: a root `CLAUDE.md` that holds more than the `@AGENTS.md` import. `fix` moves its body into `AGENTS.md` and leaves `CLAUDE.md` as `@AGENTS.md` and a newline. Claude Code reads `CLAUDE.md`. Every other agent reads `AGENTS.md`.
- `repo/budget`: a kept file over `40000` characters. It reports only. The count is characters, because a byte count inflates a file with an em dash.

The move runs even in a spec repository, because it deletes no prose. A body that `AGENTS.md` already holds is not appended again. The import line is never copied into the file it imports. A `CLAUDE.md` that is a symlink stays.

An empty `.slopfix-spec` file at the root opts a repository out of `repo/stray-markdown`. That marker is the only exemption. The answer thus lives with the repository it describes. The walk skips registered submodules, `testdata`, `.git`, `vendor`, `node_modules` and `dist`.

The submodule skip comes from `.gitmodules`, verified against the index. A declaration alone cannot exempt real source. A path that escapes the repository is an error. Outside a repository nothing is skipped. A lost derivation thus makes the check stricter.

## edit: the only way a repair writes

A repair never returns a rewritten copy of a file. It returns `edit.Edit` values, byte ranges with replacement text. The parser that owns the file then writes them through a gate. A gate checks each edit before it lands. It parses the result again and keeps an edit only when the tree still holds. A batch that fails is retried an edit at a time. A refused edit is reported in `Repair.Refused`.

| Gate | An edit may touch | After the splice |
|---|---|---|
| `treecomments.Apply` | bytes inside a comment node, and the blank around it | every node that is not a comment keeps its type, its text and its place in the tree |
| `markdown.Apply` | bytes inside a single CommonMark prose block | every verbatim block comes back as written, in order |
| workflow YAML | whole rows | the file parses, and a comment edit decodes to the same data |

So a rewrite cannot escape its comment. A newline can end a line comment early. A closer can end a block early. An opener can swallow the code below. Each changes the code tree, and the gate refuses it. The interpreter line and a cgo preamble are code to the gate, because a tool reads them.

Every repair is a `fixer.Fixer`, and each package registers its fixers from `init` with `fixer.Register`. A fixer gets a `fixer.File` and changes it only through `File.Apply` or `File.ApplyComments`. The file has no text setter. The gates are its only writers. `slopfix.Fix` opens the file for its kind and runs `fixer.For(kind)` in `Order`. `fixers_test.go` pins that order, and it fails on a repairable rule no registered fixer serves.

| Kind | Fixers, in order |
|---|---|
| source | `tombstones`, `comments/length`, `comments/number`, `comments/length-after-number` |
| document | `tombstones`, `counts/inventory-count`, `wrap-and-ste`, `ste/count` |
| workflow | `yaml/ungate`, `yaml/join-comments`, `yaml/rename-guarded-job`, `yaml/untest` |

The repository rules sit outside the registry. They delete or move whole files, and they edit no text inside one.

`edit.Scope` bounds where an edit may land, and follows the text through each pass. The hook passes the span its edit writes. `edit.Nowhere()` admits nothing, for a run that wants findings alone.

The one string match left is the hook replaying an Edit payload. `old_string` is a literal by the tool's own contract, so the hook finds it the way the tool will.

## markdown: the document model

`Split` breaks a document into prose blocks and verbatim blocks with a CommonMark parser. A fence, a table, a heading and a blank run are verbatim. No rule reads or reflows them. Every prose rule gets its fence, code and list exemptions from this split. A block keeps its list marker and indentation. A nested item thus survives a rewrite.

`wrap/hard-wrap` rejects a paragraph split over several source lines. The reader's window wraps a paragraph. An author's wrap freezes one window's width into the file. Each later edit then re-flows untouched lines.

`fix` writes each prose block back as a single line. `WordsOnly` proves the join moved only newlines. A rewrite whose words differ from the source is refused. The caller keeps the original. A workflow is never joined, because a newline in YAML is syntax.

## ste: Simplified Technical English

ASD-STE100 is a controlled language. Each approved word has a single meaning and part of speech. Its rules keep each sentence to a single reading. STE governs prose. It applies to a comment or a commit message as much as a document. The text that reaches `Check` is already a block joined to a single line.

The members share the sentence splitter, the masks and the repair pass. They therefore share a package. Each member still selects on its own by ID.

- `ste/contraction`: a contraction. The repair writes the expansion and keeps the capitalization.
- `ste/modal`: `should`, `shall`, `could`, `might` and `would`. The repair writes `must` for obligation and `can` for possibility.
- `ste/semicolon`: the semicolon. The repair writes a period and capitalizes the next word.
- `ste/comma-splice`: a comma that joins clauses that each stand alone. The repair writes a period. A connector replaces the conjunction: `However,` for `but` and `As a result,` for `so`. It drops `and`.
- `ste/sentence-length`: a sentence over `25` words, the STE cap for a description. The repair divides it at a clause boundary that the `syntax` parser finds.
- `ste/postdeterminer`: a numeral between a determiner and its noun, as in `the three rules`. The repair cuts the numeral. A unit, a percent, a year, a status code, `any` and `first` keep theirs.
- `ste/count`: a stated count anywhere in the line, read with the `Gate` substrate. The repair cuts the number after the join.

How a long sentence divides:

- A clause after `and`, `but` or `so` that names its own subject starts the new sentence.
- A verb group that shares the subject gets it again. A long subject becomes an agreeing pronoun.
- A closing `, which` clause opens with `This`. A closing `because` clause opens with `This is because`.
- A list, a quotation and a subordinate clause never divide. A sentence with no boundary is reported whole. The repair never writes a fragment.

What `ste` does not flag:

- An inline code span, a link target and an HTML entity are masked. Every repair leaves them as they are.
- A comma splice needs a subject and a finite verb after the comma. A list, an Oxford comma, a participle and an infinitive do not qualify. Without a conjunction, the words before the comma must be a main clause.
- No repair rewrites inside a quotation.
- Text in parentheses counts as a single word. A citation thus cannot inflate a sentence.
- A period ends a sentence only when what follows opens the next. That rules out `e.g.` and `$(...)`. A file name or a section mark can open a sentence in lower case.
- `ste/count` exempts arithmetic, such as a range or an expression. It exempts no noun, because a duration or a size goes stale too.

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
| Exemptions | function-word gap, a longer number | arithmetic, function-word gap | status code, exit status, literal, section sign, currency, quotation |

Prose requires a frame, because a document carries numbers that count nothing: a version, a port, an example. The gate needs no frame. That is the whole difference between the document substrates. A number beside code is nearly always a count. A comment therefore needs no frame either. The vocabularies stay separate, because widening one changes the verdict on text nobody edited. `Find` returns the whole quantity for prose. That `cardinal.Leading` can cut its number. For a comment it returns the number alone.

## counts: a count in a document

`counts/inventory-count` cuts the cardinal out of a sentence that counts what is here. `there are three sections` becomes `there are sections`, which stays true. The org rules that a count in markdown is not worth maintaining. The cut is thus the whole repair.

A count needs a frame and a quantity on the same line. The quantity is a cardinal that governs a plural noun. The frame is a possessive (`this repo's plugins`), a having verb (`it ships hooks`) or a deictic (`the rules below`). A quantity with no frame is ordinary technical prose.

It does not flag the singular, which is overwhelmingly a pronoun in English. That gap is stated rather than closed. It drops a quantity reached through a function word, as in `2 of the format drops`. It drops a match that continues a longer number, such as a version. Fences, tables, headings and backtick spans never reach it. A measurement inside a frame IS reported, because the number belongs where it is enforced.

## tombstones: a comment about a state the code has left

A tombstone describes a state the code has left, or argues for the diff instead of telling the next editor what breaks. The wording tier is the weakest. A paraphrasing machine writes the judged text. It eventually writes around a phrase rule.

- The wording tells are `pattern` entries with an `id` in `english/english.xml`. Each cuts the PHRASE, not the line. The rest of the sentence thus survives. A pattern with no `id` still rewrites.
- `tombstones/name-nothing-in-the-repository-defines` reads no wording. A comment that names a symbol found nowhere in the repository describes a tree that is gone. The probe runs ripgrep on the working tree.
- `tombstones/comment-volume` counts the lines of a merged comment run against the cap, which defaults to `14` lines. No rewording defeats it. It never strips, because it judges a whole block.

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

- `comments/number`: a number in a comment, read with the `Comment` substrate. The repair says it in words. To point at a section, cite its slug or heading, never its position.
- `comments/length`: a comment run weighed against the construct beneath it. Lines catch an essay. Characters catch a dense paragraph. The budget has a floor. A short comment is never a finding.
- `comments/tail`: a comment that stops on a word that opens what a cut took away. The repair closes the sentence.

The length repair cuts from the end, because a comment leads with its point. Each cut lands on a sentence end. The opening sentence is never cut. A block with no cut that fits is reported for a person to rewrite. The number repair runs after the length cut. The length cut then runs again if the words overflow.

It does not flag a comment above the package declaration, or a trailing comment on a code line. It skips a directive line, such as a build constraint, a shebang, a linter pragma or a cgo preamble. It skips a number inside a quotation.

## yaml: workflow and action manifest rules

These rules judge the format. The prose rules never run on these files. A file is read when its base name is `action.yml` or `action.yaml`, or when it is YAML under a `workflows` directory. Other YAML with a left-margin `jobs:` or `runs:` key qualifies too.

- `yaml/comment-block`: a run of comment lines past the single-line limit. A blank line neither counts nor ends a run. The repair joins the run into the allowed line and keeps every word.
- `yaml/all-builds-job`: a job named `all-builds`, by key or rendered name. The required gate is a commit status from the required-builds-manager app. A job with that name shadows it. The repair renames the job to `builds` and fixes each `needs` entry.
- `yaml/test-in-workflow`: a test inside a `run:` script. That is an assertion with a nonzero exit, a function whose name says it asserts, or a redirect to a test file. The repair deletes the assertion lines, because a test belongs in the suite.
- `yaml/neutered-gate`: a gate step under `continue-on-error`. A step allowed to fail is not a gate. The repair deletes that line, found by parser positions.

The all-builds wording is the operator's own. It must not be softened. A job that wears the required status's name is a known deception attempt.

Unparseable YAML yields no all-builds finding, because the runner fails on it anyway. A job name that holds an expression is skipped. A matrix suffix and a reusable workflow's path parts are stripped. A segment must then match exactly. A step that only runs a command is not a test. An error annotation alone is a report. Every repair works on whole lines and never reflows.

## laziness: a turn that ends with the work undone

`laziness/punt` reports a closing message that leaves the work undone because the work is harder. It covers a found defect left in place, and permission asked in place of action. Both leave the reader holding the work. One table therefore holds both.

The shapes are: disowning a defect you found, handing a repair back, leaving a defect in place. Excusing yourself from a repair, announcing an attribution hunt, offering authorship in place of a fix. And asking permission in place of acting.

```
$ printf 'Want me to fix it?\n' | slopfix message --json
{"findings":[{"id":"laziness/punt","tell":"asking permission in place of acting","sentence":"Want me to fix it?","line":1}]}
```

A sentence that carries a shape AND says the repair happened is not a punt. A fix claimed in another sentence pardons nothing. A bare diagnosis is legitimate. The word `pre-existing` is thus absent from the table. What fires is the diagnosis given as the reason to stop. An honest deferral that names a real blocker is clean. Fences, indented code and blockquotes are exempt. Inline backticks are not. A sentence reports a single shape.

It reports only, because the repair is work. The `no-laziness` plugin runs it at Stop, because only a Stop hook can stop the model stopping. The hook answers with the single word the reader types back. It gives nothing to argue with. The `laziness` subcommand reads the transcript when the payload carries no message.

## blamelanguage: a message that deflects blame

`blame/deflection` marks a message that deflects the work onto another author or an earlier time. It never refuses. A Stop refusal runs after the message streams. It unsends nothing. The retype then loops without a bound. The standing rule for a guard here is to mitigate rather than refuse.

The launcher execs `slopfix message --only blame` on MessageDisplay and appends one line for the reader. Nothing goes to the model. `displayContent` changes the screen and not the stored message. `CC_NO_BLAME_LANGUAGE=0` disables it. Every failure prints nothing, because a guard that eats output is worse than none.

The message is judged whole. A per-message state file keyed by `message_id` accumulates the flushes. The final flush drops it. A lost file costs earlier phrases, never a wrong mark.

`bannedPhrases` is a plain slice. It holds provenance openers such as `that predates this session` and `git blame shows`. It also holds `pre-existing` and narrow synonyms such as `not related to my change`. Matching is case-insensitive over whitespace-normalized text, with source offsets kept. Fences, indented code and blockquotes are exempt. Inline backticks are not. A real blocker named plainly is never marked.

## ask-properly, link-refs, busy-poll, md-budget

- `ask-properly` appends a line when a message puts a decision to the reader in prose. `AskUserQuestion` renders the choices instead. It refuses nothing and judges the message whole.
- `link-refs` renders a pull request, a commit or a branch as a markdown link as the message streams. A target it cannot show to exist stays plain text. A state file carries the fence state across flushes.
- `busy-poll` serves Stop and PreToolUse. On Stop it refuses a turn that repeats the same call seconds apart. On PreToolUse it refuses a status read of a subject this session saw settle. It runs in a remote session only, because waiting for an event needs events. Evidence ages out. Every failure allows.
- `md-budget` keeps each `CLAUDE.md` and each `claude_snippets/` file under `40000` characters, because every request re-sends them. It reports at session start and on a write. It blocks a Stop that leaves one broken. `CC_CLAUDE_MD_BUDGET` sets the budget. `CC_CLAUDE_MD_WIDTH` turns on the wrap check.

`english/english.xml` also links an `owner/repo#N` reference in a message. That pattern cannot check existence. The `link-refs` subcommand thus stays the verified path.

## auto-allow

`auto-allow` approves read-only work and refuses a program this environment does not run. The table is `autoallow/rules.xml`, embedded and checked by `rules.xsd`. MCP trust is per SERVER, never per tool-name pattern.

The binary is registered on PermissionRequest and PreToolUse. In `cli.js` (checked at `2.1.220`), `createCanUseTool` evaluates the rules first. PermissionRequest hooks run only on the "ask" path. `defaultMode: "auto"` answers first. A deny that rides PermissionRequest thus never fires in auto mode.

PreToolUse fires before the permission pipeline, whatever its outcome. A deny there stops the tool. The reason reaches the model as an error result. Deny therefore rides PreToolUse. Allow rides PermissionRequest, where it never outranks the user. `denyOnly` in `run.go` drops every non-deny verdict on PreToolUse. The events want different output objects. `decisionPayload` therefore branches on the event. The verdict comes from `tool_input` alone. A call judged on both events is therefore harmless.

`rules.xml` has no `<mcpServer name="Claude_Code_Remote">` block, on purpose. Do not add one. The CCR proxy answers a call with `-32003` ("needs_approval"). The client then shows an approval card and runs the hooks beside it. A hook allow calls `cancelRequest`, which kills the card and the listener that a human click resolves through. The single retry carries no grant. The model then sees `MCP error -32003: MCP tool call requires approval`.

Nothing else automates the click. The retry hands `canUseTool` a prebuilt `ask` decision. No rule, mode or classifier runs. `extraAllowedTools` does not exist in the bundle. `serverApprovalWatch` is the shape of a real upstream fix. `anthropics/claude-code#81362` tracks it. Without `send_later`, use the `/loop` skill. Watch a pull request with `mcp__github__subscribe_pr_activity`.

## no-work-loss

`no-work-loss` asks questions of one parsed command. Destruction: does it destroy content that exists only in the working tree? Provenance: does it change file content without Write, Edit or NotebookEdit?

### Destruction

The invariant: content that exists only in the working tree must never be lost by a command the agent runs. Committed work stays in the reflog. A history rewrite is thus out of scope.

The hook preserves, then allows. It commits the at-risk paths onto the CURRENT BRANCH, where the log and the next push already look. A hidden ref is one nobody reviews.

- Every step uses a throwaway `GIT_INDEX_FILE`, seeded by `git read-tree HEAD`. The real index and the working tree never move.
- `stagedTree` commits the index entries first. `git add --force` then commits the disk content. A tree equal to its parent is dropped. No commit is therefore empty.
- `git update-ref HEAD` advances the branch. `git reset -q <commit> -- <paths>` refreshes the real index for those paths. Git hooks are off during preservation.
- A best-effort `git push --no-verify origin HEAD` follows. A push failure still allows. The report names where the content sits.
- A failed `add`, `write-tree`, `commit-tree` or `update-ref` denies. Preservation that did not happen must never read as success.

A stash entry and a ref-destroying command still deny outright. A ref under `refs/no-work-loss/` is the only copy of its content. Deleting or force-pushing it is always refused.

The incident: `git checkout master` carries a dirty tree across. `git reset --hard origin/master` then destroys it with no reflog entry. A guard on `reset --hard` alone misses the setup step.

Each verb is checked against the classes it reaches. A single dirty bit gives false positives that get a guard uninstalled.

| Command | tracked edits | untracked | ignored | stash |
|---|---|---|---|---|
| `reset --hard` | destroys | spares | spares | spares |
| `clean -fd` | spares | destroys | spares | spares |
| `clean -fdx` | spares | destroys | destroys | spares |
| `stash drop` | spares | spares | spares | destroys |
| `checkout <ref>` | destroys | spares | spares | spares |

A submodule at a commit other than its gitlink is not a tracked edit. That commit stays in the history of the submodule. The probe reads `status --porcelain=v2` and drops a submodule entry that has no modified and no untracked content.

A ref-destroying verb asks whether the content exists anywhere else:

| Verb | What must survive | How it is answered |
|---|---|---|
| `branch -D` / `-M`, `update-ref -d`, `push --delete` | the tip | another ref contains it |
| `push --force` / `+refspec` | the remote-tracking tip | an ancestor of the push, or in another ref |
| `filter-branch` | all of HEAD | `rev-list --count HEAD --not --remotes` is `0` |
| `reflog expire` / `delete` | nothing reflog-only | `fsck --unreachable --no-reflogs` finds no commit |
| `worktree remove --force` | its edits | its `status --porcelain` is empty |

- Containment uses `for-each-ref --contains`. The `--exclude` flag with `--branches` takes the name without `refs/heads/`. A full refname thus excludes nothing.
- `refs/remotes/<remote>/HEAD` aliases the overwritten branch. The check filters it out.
- `push --mirror` is always refused. A push with no local remote-tracking ref denies and names `git fetch`.

Allowed on purpose: unpushed commits on a clean tree, `checkout -b`, `switch -c`, `stash push`, `commit`, `add`, `restore --staged`, unknown verbs, appends, and anything outside a repository.

### Detection

- Chains, pipes, subshells, bodies and command substitutions are each evaluated.
- A `cd` carries forward across `&&`, `||`, `;` and brace blocks. A pipe stage, a subshell and a conditional body get a copy. `git -C` and `--work-tree` apply too.
- Wrappers resolve to their program, with value-taking flags understood: `env`, `sudo`, `doas`, `command`, `builtin`, `exec`, `nohup`, `nice`, `ionice`, `setsid`, `stdbuf`, `timeout`, `xargs`. A leading `\` and an absolute path.
- Short flags unbundle. A `--flag=value` registers as `--flag`.
- Git aliases resolve from `git config`, including `!shell` aliases. Self-reference stops at depth `3`.

Ambiguity denies. That covers an unparseable command with a destructive verb, an operand that is not static (`rm $TARGET`, `cd -`), and a relocated `GIT_DIR`. A script FILE that the walk follows is judged on its static paths only.

A device target such as `/dev/null`, and a descriptor other than stdout, cannot empty a file. The provenance half still judges every descriptor. It thus refuses `echo x 2> tracked.go`.

A destructive verb fails closed on a panic, a git error or the `3`-second git timeout. Everything else fails open. A killed binary lets the command run, because a hook cannot make itself mandatory. `clean-bash` rewrites `rm`. This hook sees the original command. Where both fire, the deny wins.

### Provenance

Any change to file content in the working tree goes through Write, Edit or NotebookEdit. The verdict follows from the shape of the write:

| Shape | Example | Denies when |
|---|---|---|
| named path | `sed -i x.go`, `> x.go`, `cp a x.go` | the path is in a guarded root and not under a build directory |
| whole directory | `patch`, `tar -x`, `git apply` | the directory holds or sits in a guarded root |
| opaque | `node -e`, an `xargs`-fed `sed -i`, a GitHub API commit | always |

- Routes include editors, `busybox` applets, every file redirect, `tee`, `dd of=`, `truncate -s`, `sponge`, `xxd -r`, `sort -o`, `split`, compressors without `-c`, `zip`, `docker cp`, `yq -i` and `ln`.
- `cp`, `mv`, `install`, `rsync` and `scp` test the source side. A plain `mv old.go new.go` thus passes.
- `git apply`, `git am`, `rebase` and `revert` are refused. `git merge`, `git pull` and `git cherry-pick` pass, because git already holds what they land.
- Indirection is followed: `sh -c`, aliases, script files, `find -exec`, functions and wrappers.
- A long in-place flag (`--in-place`, `--write`) counts for any program. `allowedFormatter` lists. The tools that rewrite by design. `jq` has an empty flag set on purpose.
- A subagent spawn with a tool grant or a permissive `permissionMode` is refused. The live settings files are refused to every tool.
- The session scratchpad is the one temporary directory that does not deny.

A Write over a path that git holds and the disk does not is refused, however the path was emptied. The hook restores the file with `git restore --worktree` first. Commit the removal to free the path. `echo x > new-file` is refused too, because creating a file is what Write is for.

## clean-bash

`clean-bash` rewrites a Bash command rather than refusing it, wherever a rewrite exists. The rules run on the parse tree to a fixed point. An unparseable command passes through. A rewrite is silent: it emits only the new input and `suppressOutput: true`. A visible message gives the model something to blame. A denial carries a reason. Without one, the model retries forever. `CLEANUP_BASH_CMDS_LOG=/path` logs each rewrite and each deny.

Denials: `heredoc`, `perl` (`^perl[0-9.]*$` as the effective command), `file_read` (`cat`, `head`, `tail` or a line-selecting `sed` on a file), `shred`, `git_rm` without `--cached`, `truncate_zero` with an unknown flag, `rm_flag` for an `rm` flag it cannot drop. And `toolchain_capture` for `go-toolchain` inside a substitution.

- `devnull` removes `2>/dev/null` in any spelling. It matches a parsed redirect. A `12>/dev/null` thus stays.
- `rm_recycle`, `truncate_recycle` and `find_delete_recycle` turn deletion into `recycler trash`. Delete-then-Write is the loophole around the Write refusal.
- `head_tail`, `grep`, `or_true` and `stderr_merge` drop a trailing `| head`, `| tail`, `| grep`, `|| true` or `2>&1` from the final statement.
- `tee` turns a trailing stdout redirect on the final statement into `| tee file`.
- `toolchain_output` strips every pipe stage or stdout redirect from `go-toolchain`.
- `docker_compose_restart` writes `docker compose up -d --force-recreate`.
- `gh_wait_ci` maps `gh run view`, `watch`, `rerun`, `list` and `gh pr checks` to `gh wait-ci`.
- `sleep_cap` writes `sleep 3` for any sleep past `3` seconds or not literal.
- `narration_remove` turns an `echo` that reaches the terminal into `:`.
- `pipefail` injects `set -o pipefail`. A `tee` then never masks a failure.

## What no rule reads

A fenced code block, a table and a heading are data. No rule reads them and no repair reflows them. An HTML entity and an inline code span are masked before a rule sees the text.
