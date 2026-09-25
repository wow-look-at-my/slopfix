# slopfix: the agent reference

slopfix is one binary for the org's prose rules, the rules a code comment follows, the rules a GitHub Actions workflow follows. The rules a repository's markdown layout follows. It also carries the Claude Code hook subcommands that the marketplace plugins exec. A hook, a CI job and an editor integration all call this binary, so each gets the same verdict.

A repository keeps only `README.md` (for a person), `AGENTS.md` (this file) and `CLAUDE.md` (the line `@AGENTS.md`). Put depth here, not in a new markdown file. The `repo/stray-markdown` rule deletes any other markdown file.

## Build and test

```sh
go-toolchain            # builds build/slopfix, and runs the tests
GO_TOOLCHAIN_DATS_BUILD_DIR="$PWD/build" dats dats/no-work-loss.dats
```

`dats/` holds the CLI suites that go-toolchain runs after the build. They exec the built binary under a sandbox. So a manual run needs bubblewrap. A test that drives a foreign git hook belongs there.

`hooks.go` maps each hook to the rule IDs it runs. `hooks_test.go` asserts that every rule has a home in that map. That every entry names a rule that exists. A rule with no home fails the build. An entry for a deleted rule fails the build too.

## Commands

```sh
slopfix check [path...]           # report what the rules reject, exit 1 on any finding
slopfix check --fix [path...]     # repair in place then report what is left
slopfix fix [path...]             # the same as check --fix
slopfix fix --only ste/semicolon doc.md
slopfix check --path doc.md < doc.md   # judge text on stdin as if it were headed for doc.md
slopfix parse "The gate reads every file."
slopfix report --path doc.md < doc.md  # JSON findings, what the editor plugin reads
slopfix message < message.txt     # judge a closing message
```

- A directory argument is walked. The walk skips hidden directories (except `.github`), `vendor`, `node_modules`, `testdata`, `build`, registered submodules and nested Go modules under a module root.
- A named file is read whatever its extension. The path decides which rules read a file: a workflow or action manifest gets the `yaml` rules, a document gets the prose rules. And source gets the `comments` rules.
- A document is `.md`, `.markdown`, `.mdown` or `.txt`. An empty `--path` also counts as a document, because text with no file is prose.
- With no path argument, `check` reads stdin. Repairing, it writes the repaired text on stdout. The findings on stderr. `--json` writes the whole answer as one object instead, which is what a hook reads.
- `--max-comment-lines` sets the tombstone volume cap. `0` turns the cap off.
- `fmt`, `purge`, `comments` and `workflows` do not exist as commands. The wrap join is the `wrap/hard-wrap` rule inside `check` and `fix`. The purge is the `repo` category. The comment and workflow rules run inside `check` and `fix` on each file they judge.

### Selecting rules with --only

`--only` takes a comma-separated list. An entry is a category, or a rule ID inside a category. A rule ID is the name the report prints beside a finding, the way a compiler names a warning.

```sh
slopfix fix --only counts            # every rule in the counts category
slopfix fix --only ste/semicolon     # that rule alone
slopfix fix --only tombstones,wrap   # categories
slopfix fix --only ste/nosuch        # an error that names the rules ste holds
```

The categories are `tombstones`, `counts`, `wrap`, `ste`, `comments`, `yaml` and `repo`. A rule ID turns its category on. An unknown name is an error, because a run that applies nothing reads as a clean file. There is no `--exclude`: an exemption a caller writes is one a caller sets to everything.

A word repair and the wrap join share a pass. A rule reads a paragraph as a sentence stream, and a hand wrap hides half of it. So naming an `ste` rule also joins the paragraph it repairs.

### Other commands

- `slopfix parse [--tags] sentence...` prints the noun phrases in `[ ]`, the verb groups in `< >`, then one line per clause with its kind, depth, subject and verb.
- `slopfix report --path P [--only IDs]` reads one document on stdin and writes its findings as JSON. `--path` is required, because the path decides which rules read the text. It always exits 0, because the caller decides what a finding means.
- `slopfix hook [--only ...] [--max-comment-lines N]` reads a PreToolUse payload for Write, Edit or MultiEdit. It repairs the text the write adds, lets the write through, and flags what the repair did not reach. It prints nothing and exits 0 for anything it does not judge: an unreadable payload, another event. A tool that writes no file, or text its rules leave alone.
- The hook repairs an Edit where it lands in the file, so a line inside a fence stays code. A repair that reaches past the edit is not applied. The hook reports the findings instead.
- `slopfix message [--only ID|family] [--json]` judges a closing message on stdin. It exits 1 when a rule rejects something. The other commands judge a file. This judges the text the model sends to the reader, which is never on disk.
- The hook subcommands each read a hook payload on stdin. And write the hook's own response on stdout: `auto-allow`, `busy-poll`, `no-work-loss`, `md-budget`, `link-refs`, `clean-bash`, `ask-properly`, `laziness` and `blame-language`. A refusal is an exit code that the launcher passes through. The marketplace plugin is then a manifest and a launcher, with no rule of its own.
- `--trace` on any command prints a per-phase timing breakdown.

## Rule table

| Category | Rule IDs | Repairs |
|---|---|---|
| `repo` | `repo/stray-markdown`, `repo/agents-file`, `repo/budget` | the first two, when a directory is walked |
| `wrap` | `wrap/hard-wrap` | yes |
| `ste` | `ste/contraction`, `ste/modal`, `ste/semicolon`, `ste/comma-splice`, `ste/sentence-length`, `ste/postdeterminer`, `ste/count` | yes, except a long sentence with no clause boundary |
| `counts` | `counts/inventory-count` | yes |
| `tombstones` | `tombstones/date`, `tombstones/change-reference`, `tombstones/then-and-now-contrast`, `tombstones/position-reference`, `tombstones/hedged-time`, `tombstones/unstated-value`, `tombstones/shrug`, `tombstones/unexplained-workaround`, `tombstones/name-nothing-in-the-repository-defines`, `tombstones/comment-volume` | the wording tells and a clean whole-line referent. The volume cap reports only |
| `comments` | `comments/number`, `comments/length`, `comments/tail` | yes, except a block no cut can fit |
| `yaml` | `yaml/comment-block`, `yaml/all-builds-job`, `yaml/test-in-workflow`, `yaml/neutered-gate` | yes |
| message | `laziness/punt`, `blame/deflection` | no, `slopfix message` only |

`hooks.go` also names the `ask-properly` and `link-all-refs` hooks as pending: their detection lives in the `ask-properly` and `link-refs` subcommands rather than in a rule ID.

## CI action

The action at the repository root downloads the published binary from buildhost and runs `check` on `paths`. A consumer needs no install step.

```yml
- uses: actions/checkout@v4
- uses: wow-look-at-my/slopfix@master
  with:
    paths: .
    only: yaml/comment-block
```

`paths` defaults to `.`. So the step goes after the checkout. `only` is the `--only` flag. The action has no `command` input. It never repairs: a job that repairs its own checkout and then reports a pass has enforced nothing. On a Unix host the action runs the APE binary through `sh`, because a `binfmt_misc` handler on the MZ magic can refuse a direct exec. On Windows it execs the binary directly.

## repo: the markdown a repository keeps

These rules judge the tree rather than a file. Only a walk whose root is a repository root (a directory holding `.git`) reaches them. `check` reports them. `fix` applies them.

- `repo/stray-markdown`: a `.md` file other than the root `README.md`, `AGENTS.md` and `CLAUDE.md`. A `README.md` in a subdirectory is stray too. `fix` deletes it. Move what it says into `AGENTS.md` or `README.md` first.
- `repo/agents-file`: a root `CLAUDE.md` that holds more than the `@AGENTS.md` import. `fix` renames it to `AGENTS.md`. Or appends its body to an `AGENTS.md` that exists, and leaves `CLAUDE.md` as `@AGENTS.md` and a newline. Claude Code reads the name `CLAUDE.md` alone, and every other agent reads `AGENTS.md`.
- `repo/budget`: a kept file over `40000` characters. It reports only. The count is characters, not bytes, because a byte count reports a file with an em dash as longer than it reads.

The move runs even in a spec repository, because it deletes no prose. A body that `AGENTS.md` already contains is not appended again. And the import line is never copied into the file it imports. A `CLAUDE.md` that is a symlink stays as it is.

A repository opts out of `repo/stray-markdown` with an empty `.slopfix-spec` file at its root. That marker is the only exemption. The answer then lives with the repository it describes, not in a list here. The walk skips registered git submodules, `testdata`, `.git`, `vendor`, `node_modules` and `dist`.

The submodule skip is derived from `.gitmodules` and verified against the index. A declaration alone cannot exempt a directory that holds real source, and a path that escapes the repository is an error. A directory that is no repository skips nothing. So a lost derivation makes the check stricter, never laxer. Paths compare with symlinks resolved.

## markdown: the document model

`markdown` holds no rule of its own besides the wrap. `Split` breaks a document into prose blocks and verbatim blocks, using a CommonMark parser. A fence, a table, a heading and a blank run reach the caller marked verbatim, so no rule reflows or reads them. Every prose rule gets its fence, indented-code and list exemptions from this split. None implements the exemption itself.

A block carries its list marker and indentation, so a nested item survives a rewrite as the item it was.

`wrap/hard-wrap` rejects a paragraph split over several source lines. The reader's window wraps a paragraph. An author's wrap freezes a guess at one window's width into the file, and every later edit re-flows lines the change never touched.

```md
This is a wrapped
paragraph.
```

`fix` writes each prose block back as a single line, with its marker and indentation. `WordsOnly` proves that the join moved newlines and nothing else. A rewrite whose words differ from the source is refused rather than written: a lost word is a defect in the splitter. And the caller keeps the original.

A workflow and an action manifest are never joined. A newline there is syntax, and joining a wrapped `concurrency:` block makes GitHub reject the file before a job starts.

## ste: Simplified Technical English

ASD-STE100 is a controlled language. Each approved word has a single meaning and part of speech, and its rules keep each sentence to a single reading. STE governs prose, so the rules apply to a comment or a commit message as much as a document. Markdown is only the substrate that says where the prose is. The text that reaches `Check` is already a block joined to a single line.

The family shares the sentence splitter, the word pattern, the masks and the repair pass. So it keeps one package. Each member still selects on its own by ID.

- `ste/contraction`: a contraction. The repair writes the expansion and keeps the source's capitalization.
- `ste/modal`: `should`, `shall`, `could`, `might` and `would`. The repair writes the approved word: obligation becomes `must`, and possibility becomes `can`.
- `ste/semicolon`: the semicolon. The repair writes a period and capitalizes the next word.
- `ste/comma-splice`: a comma that joins clauses that each stand alone. The repair writes a period. A connector replaces the conjunction: `and` goes, `but` becomes `However,`, and `so` becomes `As a result,`.
- `ste/sentence-length`: a sentence over the STE description cap of `25` words. The repair divides it at a clause boundary that the `syntax` parser finds.
- `ste/postdeterminer`: a numeral between a determiner and its noun, as in `the three rules`. The repair cuts the numeral. A unit, a percent sign, a year, a status code, a quantifier such as `any` and an ordinal such as `first` keep theirs.
- `ste/count`: a stated count of items anywhere in the line, read with the `Gate` substrate in `cardinal`. The repair cuts the number, after the join.

The sentence division obeys these rules:

- A clause after `and`, `but` or `so` that names its own subject starts the new sentence.
- A verb group that shares the subject gets the subject again. A short subject repeats. A long subject becomes a pronoun that agrees with the verb.
- A closing `, which` clause becomes a sentence that opens with `This`. A closing `because` clause becomes `This is because`.
- A list, a quotation and a subordinate clause are never divided. A sentence with no boundary stays whole and is reported. The repair never writes a fragment.

```md
It should work; it does not. `a; b` stays.
```

`slopfix fix --only ste/semicolon` writes `It should work. It does not.` and leaves the modal alone, because no caller asked for it.

What `ste` does not flag:

- An inline code span, a markdown link target and an HTML entity are masked before any rule reads the text. An entity ends in a semicolon. A repair leaves each span exactly as it was.
- A comma splice needs a subject and a finite verb after the comma. A list and an Oxford comma carry no verb, and a participle or an infinitive does not mark a clause. With no conjunction, the words before the comma must be a main clause with a finite verb. So an introductory phrase is left alone.
- A quotation is somebody else's words. No repair rewrites inside one.
- Text in parentheses counts as a single word toward the cap. So a citation cannot inflate a sentence.
- A period ends a sentence only when what follows opens the next. That rules out `e.g.` and the `$(...)` of a shell example. A lower-case opener still starts a sentence when it is a file name or a section mark.
- `ste/count` exempts a number that is arithmetic, such as a range or an expression. It exempts no noun: a duration or a size states what is true today, and nothing corrects it when either moves.

## syntax: the sentence parser

`syntax` holds no rule ID. It parses an English sentence into phrases and clauses, for the cases a regular expression cannot tell apart. The `ste` repairs use it for the numeral rule and for the clause boundary where a long sentence divides.

1. The `prose` tokenizer splits the sentence into words. Each word keeps its byte span in the source.
2. The `prose` averaged-perceptron tagger gives each word a Penn Treebank tag. Vale uses the same tagger.
3. A retag pass corrects the tags that a clause test depends on. The tagger reads "the write fails" as a plural compound noun, so a stretch with no finite verb gets its verb back.
4. Chunking groups the words into noun phrases and verb groups. A noun phrase records its determiner, the numerals after it and its head.
5. The clause pass cuts at conjunctions, subordinators, relative words and punctuation. Each clause gets a subject, a finite verb and a depth. A main clause has depth zero.

A word inside an opaque span reads as a name. The rules pass code spans, parentheticals and quotations this way, because their words are data. The word classes live in `rules/syntax.xml`. `github.com/jdkato/prose/v3` is MIT.

```
[The gate] <reads> [every file] and <refuses> [the write] .
opens/0 subject="The gate" verb="reads": The gate reads every file
coordinate/0 subject="-" verb="refuses": and refuses the write
```

## cardinal: is this number a count

`cardinal` holds no rule ID. It decides whether a number is a stated count: true today and wrong after the next commit. Each caller brings its substrate. And the substrate is the policy.

| | `Prose` | `Gate` | `Comment` |
| --- | --- | --- | --- |
| Rule | `counts/inventory-count` | `ste/count` | `comments/number` |
| Shape | quantity | quantity | number |
| Frame required | yes | no | no |
| Vocabulary | two upward, plus a dozen | two to twelve | cardinals, ordinals, scales, repeat counts |
| Exemptions | function-word gap, a longer number | arithmetic, function-word gap | status code, exit status, literal, section sign, currency, quotation |

- Prose requires a frame, because a document carries numbers that count nothing: a version, a port, an example. The frame is a sentence claiming the things belong here.
- The gate asks for no frame, so a bare quantity anywhere in the line is a finding. The frame is the whole difference between the document substrates.
- A comment asks for no frame, because a number beside code is nearly always a count of what the code holds. A URL is never a count. And a qualified name names itself.
- The vocabularies are not derived from each other. Widening one changes the verdict on text nobody edited, so they stay separate lists.

`Find` returns what the substrate's repair needs. Prose returns the whole quantity, so `cardinal.Leading` can cut the number off its front. A comment returns the number alone. `cardinal.IsUnit` names a unit of measure, and `ste/postdeterminer` is its caller.

## counts: a count in a document

`counts/inventory-count` cuts the cardinal out of a sentence that states a count of what is here. `there are three sections` becomes `there are sections`. Which stays true through the next commit. The org rules that maintaining a count in markdown is not work worth doing. So the cut is the whole repair.

A count qualifies only when a frame and a quantity meet on the same line of prose. The quantity is a cardinal governing a plural noun, with adjectives allowed between them. The frame is a possessive (`this repo's plugins`), a having verb (`it ships hooks`, `there are sections`), or a deictic into the page (`the rules below`). A quantity with no frame is ordinary technical prose.

What `counts` does not flag:

- The singular. In English prose it is overwhelmingly a pronoun. A page that says `one plugin` is still wrong after one edit. And that gap is stated rather than closed.
- A quantity reached through a function word. Without that filter, `2 of the format drops` reads as a count of drops.
- A match that continues a longer number. So a version string is not a count.
- A fence, an indented block, a table and a heading. A backtick span is blanked inside its line, because a cardinal in verbatim machinery is a literal.

A measurement inside a frame IS reported here. A budget gets raised and a suite gets slower, and the number belongs where it is enforced. The cut runs back to front, so an earlier span's offsets stay valid.

## tombstones: a comment about a state the code has left

A tombstone describes a state the code has left, or argues for the diff instead of telling the next editor what breaks. The family has tiers. And the wording tier is the weakest. A paraphrasing machine writes the judged text. So it eventually writes around any rule keyed to a phrase.

- The wording tells are `pattern` entries with an `id` in `english/english.xml`. Each cuts the PHRASE rather than the line, so the sentence's other half survives and the write lands finished. An entry with no `id` still rewrites and counts toward `Rewrites`.
- `tombstones/name-nothing-in-the-repository-defines` reads no wording. A comment that names a symbol found nowhere in the repository describes a tree that is gone. The probe runs ripgrep against the working tree.
- `tombstones/comment-volume` is the tier no rewording defeats. It counts the lines of a merged comment run against the cap, which defaults to `14` lines. It is a judgement about a whole block, so it never strips.

The referent tier exists for a comment with no tell in its phrasing: `see TestDarwinStatfsToLinux for the pin`, written beside the change that deleted that test. The volume tier exists because a comment can be surplus while every sentence passes a wording rule.

What `tombstones` does not flag:

- Only prose is judged. In source that means the comments, so a string literal holding `previously` is not a tombstone. In a document it means the paragraphs, with fences, HTML comments, frontmatter and indented blocks skipped, and backtick spans blanked.
- A referent candidate must look like a symbol: an underscore, an internal case change, or a leading capital beside a digit. A name in capitals and a short name never qualify. Reporting every name a low-level comment mentions is how a guard gets turned off.
- The probe answers nothing when it cannot answer: no repository, no ripgrep, a timeout or a failed search. An absent answer must never read as a missing symbol. A comment that puts too many names to the repository is skipped for the same reason.
- A file whose extension the scanner has no syntax for is not judged. A guess at syntax risks reading code as prose.
- A document carries no volume cap. A long paragraph there is ordinary writing.

A referent finding strips only when its line is the raw source line the comment sits on and nothing else shares it. A trailing comment on a code line stays reported. Every finding in a document is unstrippable, because a document line is a paragraph. After a strip, the surviving block is rewrapped. A deleted line is named in the response and nowhere else. Git history holds anything worth keeping.

`--path` is required for this family on stdin, because the path decides the comment syntax.

```sh
slopfix fix --only tombstones --path pkg/thing.go < pkg/thing.go
slopfix fix --only tombstones --max-comment-lines 0 --path pkg/thing.go < pkg/thing.go
```

## comments: what a comment gets wrong

The `commentfix` package holds every rule a source comment answers to. It reads a real syntax tree through `go-tree-sitter`. A comment is a node whose type carries `comment`. And the documented construct is the next named sibling. Nothing in the rules names a language. A file that does not parse yields nothing, because half a tree is a worse input than none.

The languages are Go, C, C++, Rust, Bash, JavaScript, TypeScript and TSX, by extension. YAML. TOML, `.conf` and `.zsh` files read with the Bash grammar. `languages_test.go` refuses a grammar. That no fixture proves the repair works on. `selfrepair_test.go` runs the rules over this tree.

- `comments/number`: a number in a comment, read with the `Comment` substrate. A number there is a count of what exists today. And the edit that adds an item leaves it wrong. The repair says it in words. To point at a spec section, cite its slug or heading, never its position.
- `comments/length`: a run of comment lines weighed against the construct beneath it. Lines catch an essay, and characters catch a dense paragraph. Either measure alone reports the block. The budget has a floor. So a short comment is never a finding.
- `comments/tail`: a comment that stops on a word that opens what a cut took away. The repair closes the sentence.

The length repair cuts from the end, because a comment leads with its point. A paragraph goes before a sentence does. Every cut lands on a sentence end. The opening sentence is never cut. A block with no cut that both fits and reads is reported as unrepairable, for a person to rewrite. The number repair runs after the length cut. And the length cut runs again when the words put a block back over budget.

What `comments` does not flag:

- A comment above the package declaration. It introduces the package, so nothing of comparable size stands beneath it.
- A trailing comment that shares its line with code. Measuring counts the code, and cutting deletes it.
- A directive line: a build constraint, an interpreter line, a linter pragma or a cgo preamble. It addresses a tool.
- A number inside a quotation.

go-toolchain's vet phase runs these rules on every build.

## yaml: workflow and action manifest rules

These rules are about the format, not the prose inside it, and the prose rules never run on a workflow or a manifest. A file is read when its base name is `action.yml` or `action.yaml`, or when it is a YAML file under a `workflows` directory. Any other YAML file with a left-margin `jobs:` or `runs:` key is sniffed as one too.

- `yaml/comment-block`: a run of comment lines past the limit of a single line. A blank line neither counts toward a run nor ends one. The repair joins the run into the one allowed line and keeps every word.
- `yaml/all-builds-job`: a job named `all-builds`, by key or by rendered name. The org's required gate is a commit status from the required-builds-manager app. A job with that name satisfies nothing and shadows the real gate in the UI. The repair renames it to `builds` and fixes each `needs` entry that points at it.
- `yaml/test-in-workflow`: a test inside a `run:` script. That is an assertion pairing a comparison with a nonzero exit, a shell function whose name says it asserts. Or a redirect naming a test file. The repair deletes the assertion lines and keeps the step. The suite is where a test belongs.
- `yaml/neutered-gate`: a step that runs a gate under `continue-on-error`. A step allowed to fail is not a gate. The repair deletes the `continue-on-error` line, which it finds by parser positions.

The all-builds wording is the operator's own. And must not be softened: a job that wears the required status's name is a known deception attempt.

What `yaml` does not flag:

- Unparseable YAML yields no all-builds finding. The runner cannot read that file either. So it fails on its own.
- A job name holding an expression resolves at run time. So it is skipped. A matrix suffix in parentheses and a reusable workflow's path parts are stripped first. Then a segment must match exactly. `all-builds2` is a different name.
- A step that only runs a command is not a test. It fails on its own exit code. An error annotation alone is a report, not an expectation.
- Nothing is reflowed, because a newline in YAML is syntax. Every repair works on whole lines.

## laziness: a turn that ends with the work undone

`laziness/punt` reads a closing message and reports a turn that ends with the work undone because the work is the harder path. It covers a defect the session found and left in place, and a request for permission in place of action. Both leave the reader holding the work, so one table holds both shapes.

The shapes are: disowning a defect you found, handing a repair back to the reader, leaving a defect in place. Excusing yourself from a repair, announcing an attribution hunt, offering authorship in place of a fix. And asking permission in place of acting.

```
$ printf 'I found a broken build phase. Neither mine to fix unasked.\n' | slopfix message
1: [laziness/punt] disowning a defect you found: "Neither mine to fix unasked."
exit=1
$ printf 'Want me to fix it?\n' | slopfix message --json
{"findings":[{"id":"laziness/punt","tell":"asking permission in place of acting","sentence":"Want me to fix it?","line":1}]}
```

What it does not flag:

- A sentence that carries a shape AND says the repair happened. A fix claimed in a different sentence pardons nothing.
- A bare diagnosis. Naming a failure as pre-existing is real work. So that word is absent from the table. What fires is the diagnosis offered as the reason to stop.
- An honest deferral that names a real blocker and says what was pushed in its place.
- Fenced code, indented code and a blockquote. So the policy can be written down. An inline backtick span is not exempt.

A sentence is reported for a single shape. It reports only, because the repair is work rather than words. The `no-laziness` plugin runs it at Stop. The wording guards moved to MessageDisplay, but this rule exists to stop the model STOPPING, and only a Stop hook can do that. The hook answers with the single word the reader types back, and gives nothing to argue with. The `laziness` hook subcommand reads the message from the transcript when the payload carries none.

## blamelanguage: a message that deflects blame

`blame/deflection` marks a closing message that deflects the work onto another author or an earlier time. It never refuses. A Stop refusal runs after the message has streamed, so it unsends nothing. The retype names the phrase again. And the loop has no bound. The standing rule for a guard here is to mitigate rather than refuse.

The launcher execs `slopfix message --only blame` on MessageDisplay and appends one line to what the reader sees. Nothing goes back to the model. `displayContent` replaces the delta on screen without changing the stored message. `CC_NO_BLAME_LANGUAGE=0` disables it. Every failure path prints nothing, which leaves the original text, because a guard that can eat output is worse than none.

The message is judged whole. A phrase can span a flush. The launcher accumulates the text in a per-message state file keyed by `message_id` and drops it on the final flush. A lost file costs the earlier phrases, never a wrong annotation. The non-streaming path is one call with `index` 0 and `final` true.

- `bannedPhrases` is a plain slice. It holds the provenance openers (`that predates this session`, `this was existing code`, `i only copied it`, `git blame shows`), `pre-existing`, `preexisting`, and narrow synonyms such as `not related to my change` and the `not my *` family.
- Matching is case-insensitive over whitespace-normalized text, so a phrase split by a wrap still matches. Normalization keeps each byte's source offset.
- Fenced code, indented code and blockquotes are exempt. Inline backticks are not.
- Naming a real blocker plainly is never marked. The rule is about deflection, not about admitting a limit.

## ask-properly and link-refs: MessageDisplay marks

- `ask-properly` appends one line when a closing message puts a decision to the reader in prose. `AskUserQuestion` renders the choices instead. It refuses nothing and sends nothing to the model. It judges the message whole on its last flush.
- `link-refs` rewrites a pull request, a commit or a branch as a markdown link while the message streams. A reference whose target it cannot show to exist stays plain text: a. Link asks the reader to move a hand before knowing whether it pays. A per-message state file carries the fence state across flushes, and a sweep removes stale files.

`english/english.xml` also links an `owner/repo#N` reference in a message. That pattern cannot check existence. So it is for a reference already known good. `link-refs` stays the verified path.

## busy-poll

`busy-poll` serves Stop and PreToolUse from one command. And the event is on the payload. On Stop it refuses a turn that is the latest of several that make the same call seconds apart. On PreToolUse it refuses a status read whose subject this session already watched settle. A properly spaced watch loop is not refused. Both rules run in a remote session only, because "wait for the event" needs events. Evidence ages out, so a verdict lapses. Every failure path allows. The refusal names the repeated call and the ways out.

## md-budget

`md-budget` keeps every `CLAUDE.md` and every snippet under `claude_snippets/` inside a character budget of `40000`. Nothing truncates those files, so every excess character is re-sent on every request. It reports at session start, again when such a file is written, and blocks a Stop that leaves one broken. `CC_CLAUDE_MD_BUDGET` sets the budget. A file near the wall is flagged too. The hard-wrap width check stays off unless `CC_CLAUDE_MD_WIDTH` is truthy. The full scan skips `.git` and `node_modules`.

## auto-allow

`auto-allow` approves read-only work and refuses a program this environment does not run. The rule table is `autoallow/rules.xml`, embedded and validated by `rules.xsd`. MCP trust is per SERVER, never per tool-name pattern.

The binary is registered on PermissionRequest and PreToolUse, and each event reaches the call at a different point. In `cli.js` (checked at `2.1.220`), `createCanUseTool` evaluates the permission rules first and returns on allow. Or deny. `executePermissionRequestHooks` runs only on the "ask" path. `defaultMode: "auto"` answers before that path. A PermissionRequest-only deny rule never fires under auto mode.

PreToolUse fires before the permission pipeline, whatever the permission outcome. A deny there stops the tool, and `permissionDecisionReason` reaches the model as an `is_error` tool result. So deny rides PreToolUse and must always fire. Allow rides PermissionRequest and must never outrank the user. `denyOnly` in `run.go` suppresses every non-deny verdict on the PreToolUse path. An allow there settles the call before the permission engine runs.

The CLI rejects a payload whose `hookEventName` differs from the dispatched event. The events want different objects. `decisionPayload` branches on the event for this reason.

```jsonc
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"..."}}
{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"deny","message":"..."}}}
```

The binary is stateless and derives its verdict from `tool_input` alone, so evaluating one call on both events is harmless.

### No Claude_Code_Remote block, deliberately

`rules.xml` has no `<mcpServer name="Claude_Code_Remote">` block. Such an entry made every `mcp__Claude_Code_Remote__*` call fail. Do not add it back.

1. Local rules allow, and `tools/call` goes out.
2. The CCR MCP proxy answers JSON-RPC `-32003` ("needs_approval") with an `args_sha256` in `error.data`.
3. The client raises a retroactive approval card whose precomputed decision (`behavior: "ask"`, `suppressAlwaysAllowRule: true`) goes straight to `canUseTool`.

For a top-level call, the client sends the card first and runs the hooks beside it. A hook that answers "allow" calls `cancelRequest`, which kills the card and the listener that a human click resolves through. That listener is the only path that yields a decision with `source: { type: "user" }`. The single retry then carries no grant, and the second `-32003` reaches the model (`mcp_ccr_needs_approval`, `retry_failed`). The user sees a card vanish, then `MCP error -32003: MCP tool call requires approval`.

Nothing else automates the click. The retry hands `canUseTool` a prebuilt decision, so no permission rule, mode or classifier is consulted. A PermissionRequest hook and an SDK `canUseTool` callback cancel the request rather than complete it. `extraAllowedTools` has no occurrence in the bundle. No request body carries an allowlist. `serverApprovalWatch` is the shape a real upstream fix takes. Upstream: `anthropics/claude-code#81362`.

Without `send_later`, use the `/loop` skill for a cadence. Watch a pull request with `mcp__github__subscribe_pr_activity`, and list repositories with `gh`. `register_repo_root` is gated too, so a clone's `CLAUDE.md`, skills and plugins do not load.

## no-work-loss

`no-work-loss` asks questions of one parsed command. Destruction: does it destroy content that exists only in the working tree? Provenance: does it change file content without Write, Edit or NotebookEdit?

### Destruction: the invariant

Content that exists only in the working tree, modified or untracked, must never be lost by a command the agent runs. Committed work stays reachable from the reflog, so rewriting committed history is not this hook's problem.

The hook preserves, then allows. It commits the at-risk paths onto the CURRENT BRANCH, where the log, the diff and the next push already look. A hidden ref is one nobody reviews and the next session deletes.

- Every step runs against a throwaway `GIT_INDEX_FILE`. The real index is never written and the working tree never moves. `git read-tree HEAD` seeds the temp index. A repository with no HEAD gets a parentless commit.
- `stagedTree` writes the at-risk paths' index entries first, then `git add --force` writes the working-tree content. A tree equal to its parent is dropped. So the ordinary case makes a single commit and never an empty one.
- `git update-ref HEAD` advances the branch. `git reset -q <commit> -- <paths>` refreshes the real index for those paths, or the tree reads as a staged revert. Git hooks are disabled during preservation.
- A best-effort `git push --no-verify origin HEAD` gets the content off the machine. A push failure still allows. And the report says where the content sits.
- The saved paths leave `git status`. A file staged elsewhere keeps its staged blob. `preserve_test.go` pins both halves.
- A failed `add`, `write-tree`, `commit-tree` or `update-ref` falls back to the denial. Preservation that did not happen must never read as success.

A stash entry and every ref-destroying command still deny outright, because they ask a different question. A ref under `refs/no-work-loss/` is the only copy of what it holds. So `update-ref -d` and a force or delete push that names one are refused unconditionally.

The incident this was built for: `git checkout master` carries a dirty tree onto master, and `git reset --hard origin/master` then destroys it with no reflog entry. The first command destroys nothing. It is what makes the second lethal.

The verbs do not agree on what "dirty" means, so each verb is checked only against the classes it reaches. A single dirty bit gives false positives that get a guard uninstalled.

| Command | tracked edits | untracked | ignored | stash |
|---|---|---|---|---|
| `reset --hard` | destroys | spares | spares | spares |
| `clean -fd` | spares | destroys | spares | spares |
| `clean -fdx` | spares | destroys | destroys | spares |
| `stash drop` | spares | spares | spares | destroys |
| `checkout <ref>` | destroys | spares | spares | spares |

Ref-destroying verbs ask whether the content exists anywhere else:

| Verb | What must survive | How it is answered |
|---|---|---|
| `branch -D` / `-M` | the branch tip | another ref contains it |
| `push --force` / `+refspec` | the remote-tracking tip | an ancestor of the push, or another ref contains it |
| `push --delete` | the remote-tracking tip | another ref contains it |
| `update-ref -d` | the ref's tip | another ref contains it |
| `filter-branch` | all of HEAD | `rev-list --count HEAD --not --remotes` is `0` |
| `reflog expire` / `delete` | nothing reflog-only | `fsck --unreachable --no-reflogs` finds no commit |
| `worktree remove --force` | that worktree's edits | its `status --porcelain` is empty |

- Containment uses `for-each-ref --contains`. `--exclude` with `--branches` or `--remotes` matches the name without its `refs/heads/` prefix, so a full refname silently excludes nothing.
- `refs/remotes/<remote>/HEAD` is a symbolic alias for the branch being overwritten. So it is filtered out.
- `push --mirror` is refused unconditionally, because no bounded set of commits can be checked.
- A push with no local remote-tracking ref denies. The fix named is `git fetch`. The denial recommends `--force-with-lease`.

Deliberately allowed: unpushed commits on a clean tree, `checkout -b` and `switch -c`, `stash push`, `commit`, `add`, `restore --staged` without `--worktree`, read-only and unknown verbs, appending (`>>`, `tee -a`, `git rm --cached`). And anything outside a git repository.

### Detection

- Chains, pipes, brace blocks, subshells, conditional and loop bodies, function bodies and command substitutions are each evaluated.
- A `cd` carries forward where the shell shares a working directory: `&&`, `||`, `;` and brace blocks. A pipe stage, a subshell and a conditional body each get a copy. `git -C` and `--work-tree` apply the same way.
- Wrappers resolve to their program: `env`, `sudo`, `doas`, `command`, `builtin`, `exec`, `nohup`, `nice`, `ionice`, `setsid`, `stdbuf`, `timeout`, `xargs`, a leading `\` and an absolute path. Value-taking flags are understood.
- Short flags are unbundled, `--flag=value` registers as `--flag`, and `--` separates operands.
- Git aliases resolve from `git config --get-regexp '^alias\.'`, including the `!shell` form. A builtin verb skips the lookup. Self-reference stops at depth `3`.

Ambiguity denies: an unparseable command that names a destructive verb. An operand that is not statically known (`rm $TARGET`, `cd -`), and `GIT_DIR`, `GIT_WORK_TREE` or `--git-dir`. A script FILE the walk follows is the exception. Its variables and working directory are its own, so only its static paths are judged. A script that does not parse still denies.

A device target (`/dev/null`, `/dev/stdout`, `/dev/stderr`, `/dev/tty`, `/dev/fd/*`) and a descriptor other than stdout cannot empty a file, so the destruction half skips both. The provenance half judges every descriptor, so `git status 2>/dev/null` runs and `echo x 2> tracked.go` is refused.

Inside the process, a destructive verb fails closed: a panic becomes a denial. A git call that errors or passes its `3`-second timeout denies. Everything else fails open. A killed or hung binary lets the command proceed, because a hook cannot make itself mandatory. The prefilter scans for `git`, `rm`, `mv`, `>`, `tee` and `truncate`, and returns before a parse otherwise.

`clean-bash` rewrites `rm` to `recycler trash`, but hooks receive the original input, so this hook evaluates `rm` as written. Where both fire, the deny wins.

### Provenance: write routes

Any change to file content under the working tree goes through Write, Edit or NotebookEdit. The verdict follows from the shape of the write:

| Shape | Meaning | Denies when |
|---|---|---|
| named path | the target is on the argv (`sed -i x.go`, `> x.go`, `cp a x.go`) | the path is inside a guarded root and not under a build directory |
| whole directory | the write lands under a directory (`patch`, `tar -x`, `git apply`, `split`) | that directory contains or sits inside a guarded root |
| opaque | the target cannot be resolved (`node -e`, an `xargs`-fed `sed -i`, a GitHub API commit) | always |

- Editors: `ed`, `ex`, `vi`, `vim`, `nvim`, `emacs`. `busybox` resolves to its applet. `jq` has an empty flag set on purpose. So nobody adds a write to it.
- Redirection and copy-over: `>`, `>>`, `>|`, `&>`, `<>`, `>&file`, `tee`, `dd of=`, `truncate -s`, `sponge`, `xxd -r`, `base64` and `openssl -out`.
- `cp`, `mv`, `install`, `rsync` and `scp` test the source side. Bytes already in the tree went through an edit tool, so `mv old.go new.go` passes.
- Patches: `patch`, `git apply`, `git apply --cached`, `git am`. `git merge`, `git pull` and `git cherry-pick` pass, because each lands a commit git already holds. `rebase` and `revert` stay refused. Branch navigation and `--abort`, `--continue`, `--quit`, `--skip` pass.
- A git verb with no route is checked against the alias table first, with the same `aliasResolver` as the destruction half.
- Indirection is followed: `sh -c`, alias definitions, shell script files, `./script.sh` with a shell shebang, `find -exec`, function bodies and wrappers. A missing script writes nothing, and an unreadable one denies.
- `ln` is judged on the link name. Routes found by audit: `sort -o`, `split`, `csplit`, compressors without `-c`, `zip`, `docker cp`, remote `scp` and `yq -i`.
- A long in-place flag (`--in-place`, `--write`) counts for any program. Short `-i` and `-w` count only for tools known to use them. `allowedFormatter` lists the tools that rewrite by design. Each writing only a canonical reformat. Or an owned regeneration.
- A subagent spawn with a tool grant or a permissive `permissionMode` is refused. The live settings files, and the skills that rewrite them, are refused to every tool. A plugin's own source is ordinary work.
- The session scratchpad is the one temporary directory that does not deny.

A Write over a path with content replaces work nobody read a diff of. Emptying the path first (`mv`, `rm`, `git rm`) and sending the Write again is judged on state, not on the verb: git holds content at the path. The disk does not. The hook restores the file with `git restore --worktree` before it answers. Committing the removal is the honest way out. A rename of an untracked file is a stated gap.

`echo x > new-file` is refused, even though nothing is lost, because creating a file is what Write is for. This half does not cover running programs, deletion or file metadata.

## clean-bash

`clean-bash` rewrites a Bash command rather than refusing it, wherever a rewrite exists. The rules run on the parsed tree to a fixed point, bounded at `20` passes. A non-convergence goes to stderr. An unparseable command passes through.

It is silent by design: a rewrite emits only the new input and `suppressOutput: true`, never a `systemMessage` or `additionalContext`. A visible message gives the model something to blame for its own mistakes. A denial carries a `permissionDecisionReason`, or the model retries forever. `CLEANUP_BASH_CMDS_LOG=/path` logs each rewrite with its rule names, each deny and each fail-open.

Denials, and the log reason each carries:

- `heredoc`: any heredoc. Use Write.
- `perl`: an effective command matching `^perl[0-9.]*$`. `grep perl` and `perlcritic` run.
- `file_read`: `cat`, `head`, `tail` or a line-selecting `sed` on a real file. Use Read.
- `shred`: `shred` or `srm`.
- `git_rm`: `git rm` without `--cached`.
- `truncate_zero`: an emptying `truncate` with a flag it does not translate.
- `rm_flag`: `rm` with a flag other than `-r`, `-R`, `-f`, `-v`, `-i`, `-I` or their long forms.
- `toolchain_capture`: `go-toolchain` inside `$(...)` or `<(...)`. Dropping the capture changes what the script computes.

Rewrites:

- `devnull`: a `2>/dev/null` in any spelling is removed, anywhere. The match is a parsed redirect, so `12>/dev/null` and a quoted string stay.
- `rm_recycle`, `truncate_recycle`, `find_delete_recycle`: deletion becomes `recycler trash`, tree-wide. Delete-then-Write is the loophole around the Write refusal. And the gap holds the only copy. The rule never emits `purge` or `empty`.
- `head_tail`, `grep`, `or_true`, `stderr_merge`: a trailing `| head`, `| tail`, `| grep`, `|| true` or `2>&1` on the final statement is dropped. Mid-script and inside `$(...)` they stay.
- `tee`: a trailing stdout file redirect on the final statement becomes `| tee file` or `| tee -a file`. `/dev/` targets, process substitutions and captures stay.
- `toolchain_output`: a pipe stage or stdout redirect on `go-toolchain` is stripped everywhere, because its whole output belongs in the transcript.
- `docker_compose_restart`: `docker compose restart` becomes `docker compose up -d --force-recreate`, with services and flags kept.
- `gh_wait_ci`: `gh run view`, `watch`, `rerun`, `list` and `gh pr checks` become the matching `gh wait-ci` form. Any other form is left alone.
- `sleep_cap`: a `sleep` whose literal durations sum past `3` seconds, or that is not literal, becomes `sleep 3`.
- `narration_remove`: an `echo` or `printf` whose output reaches the terminal becomes `:`.
- `pipefail`: `set -o pipefail` is injected, so a `tee` never masks the producer's status.

## What no rule reads

A fenced code block, a table and a heading are data. No rule reads them and no repair reflows them. An HTML entity and an inline code span are masked before a rule sees the text, and every repair leaves them as they are.
