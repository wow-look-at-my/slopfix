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

| Category | Rule IDs | Autofix |
|---|---|---|
| `repo` | `repo/agents-file`, `repo/budget`, `repo/package-scripts`, `repo/binary` | yes, on a walk |
| `repo` | `repo/near-duplicate` | yes, on a walk. A copy loses the blocks it shares |
| `wrap` | `wrap/hard-wrap`, `wrap/long-block` | yes |
| `ste` | `ste/contraction`, `ste/modal`, `ste/semicolon`, `ste/comma-splice`, `ste/sentence-length`, `ste/postdeterminer`, `ste/count` | yes |
| `english` | `english/comma-never` | yes. `, never` becomes `, not`. Edited English rarely writes the first and often the second. Before a verb it becomes `, and never`, or `, do not` before an instruction |
| `ste`, warnings | `ste/instruction-length`, `ste/passive`, `ste/noun-cluster`, `ste/tense`, `ste/dictionary`, `ste/paragraph-length` | yes. A warning fails no check, and each one reports only where its rewrite applies |
| `counts` | `counts/inventory-count`, `counts/section-number` | yes |
| `tombstones` | `tombstones/date`, `tombstones/change-reference`, `tombstones/then-and-now-contrast`, `tombstones/position-reference`, `tombstones/hedged-time`, `tombstones/unstated-value`, `tombstones/shrug`, `tombstones/unexplained-workaround`, `tombstones/name-nothing-in-the-repository-defines`, `tombstones/comment-volume` | yes |
| `comments` | `comments/number`, `comments/length`, `comments/tail` | yes |
| `yaml` | `yaml/comment-block`, `yaml/all-builds-job`, `yaml/neutered-gate`, `yaml/env-indirection`, `yaml/push-tags`, `yaml/org-action-ref`, `yaml/concurrency` | yes |
| `yaml`, warnings | `yaml/test-in-workflow` | yes. The lines that carry the test come out of the `run:` script |
| `pins` | `pins/download-version` | yes |
| message | `laziness/punt`, `blame/deflection`, `ask/prose-decision` | yes, with `fix --message`. Each sentence that carries the tell comes out |

`hooks.go` also lists `link-all-refs` as pending. Its detection lives in the `link-refs` guard, not in a rule ID.

## Every detection has an autofix

Every rule is one `RuleSpec` in a `rule_*.go` file: a `Detect`, an `Autofix`, and the `Cases` that prove both. `RegisterRule` panics at init on a rule with no `Autofix`, so a binary with such a rule does not start. No field, flag or option exempts a rule. Every finding passes through `Registered` on its way out of `CheckContent`, `Fix` and a tree walk. A finding under an ID that no rule registers panics there. A detection thus cannot reach a reader without the autofix and the test of its rule.

A rule reports only what its repair rewrites. Where no rewrite can say a thing correctly, the rule does not report it: `ste/dictionary` reports only the words of the `plain` list in `rules/ste-words.xml`. `ste/passive` reports only a passive that names its actor.

`ruleharness_test.go` drives every case of every rule through the same common test. The test is non-negotiable, and its steps are exactly these:

```
* <input fixture, which includes the case(s) that the rule should detect>
* run ALL OTHER rules on the fixture, confirming that there are no detections other than the current rule we're testing
* run the rule on the fixture, confirming it is detected
* run the rule on the result, confirming it was fixed (i.e. no longer detected)
* run ALL OTHER rules on the "fixed" fixture, confirming there are STILL no detctions.
```

A fixture is a repository and a closing message. `Materialize` writes the repository to a temporary directory under `git init`. Every rule reads every text of it.

`ste/sentence-length` divides every long sentence into grammatical sentences, behind a carrier where no clause boundary serves (`docs/ste-simplified-technical-english.md`). `ste/semicolon` repairs every semicolon. `allfix_test.go` runs `fix` over a tree of each rule's hardest case and requires a clean `check` after it. `repairable_test.go` fails on a rule with no fixer and no repository pass behind it. `Fix` runs the fixers again until the text holds, because one repair can hand a later rule new text.

## CI action

The action at the repository root downloads the published binary from buildhost and runs `check .` with every rule. The step therefore goes after the checkout.

```yml
- uses: actions/checkout@v4
- uses: wow-look-at-my/slopfix@master
```

Several org actions call this action, so one workflow run can reach it from many jobs. The check claims the run through `wow-look-at-my/actions@run-once#latest` with the name `slopfix-check`. One job runs it. Each other job logs the skip.

No input narrows the check. A path list, a rule list or a raw command line lets a caller set the gate to nothing. `action_test.go` fails the build on any input. The action never repairs. A job that repairs its own checkout and then passes has enforced nothing.

In a fork, only the lines the fork wrote can fail the check. The org's `dot_github` repository lists, in `fork-list/fork-of.json`, each fork GitHub does not record, and publishes it to buildhost. `docs/fork-scope.md` holds the detail.

`check` and `fix` also take `--staged` and `--diff REV`. Each scopes a run to the lines a change introduced. A repair then stays off the upstream lines. [docs/commands.md](docs/commands.md) holds the detail.

## The marketplace follows each publish

cc-marketplace ships this binary inside its `slopfix` plugin. A publish here therefore reaches nobody until that plugin is packaged again. The last step of `ci.yml` does that on each master build. It dispatches `release.yml` in cc-marketplace with `publish: true`. The token is `CC_MARKETPLACE_DISPATCH_TOKEN` from secret-server, with `actions: write` on cc-marketplace. A missing token fails the build, because a silent skip leaves the marketplace on an old binary.

## repo: what a repository keeps

[docs/repo-what-a-repository-keeps.md](docs/repo-what-a-repository-keeps.md) holds this section.

## edit: the only way a repair writes

[docs/edit-the-only-way-a-repair-writes.md](docs/edit-the-only-way-a-repair-writes.md) holds this section.

## markdown: the document model

`Split` breaks a document into prose blocks and verbatim blocks with a CommonMark parser. A fence, a table, a heading and a blank run are verbatim. So is a paragraph with no letter, which is a picture, and a line that holds a template tag (`{%`, `{{`, `{#`). A template engine reads that line, and the tag line divides the paragraph around it. No rule reads or reflows them. Every prose rule gets its fence, code and list exemptions from this split. A block keeps its list marker and indentation. A nested item thus survives a rewrite.

`wrap/hard-wrap` rejects a paragraph split over several source lines. It reports each continuation line. The reader's window wraps a paragraph. An author's wrap freezes one window's width into the file. Each later edit then re-flows untouched lines.

`fix` writes each prose block back as a single line. `WordsOnly` proves the join moved only newlines. A rewrite whose words differ from the source is refused. The caller keeps the original. A workflow is never joined, because a newline in YAML is syntax.

`wrap/long-block` rejects a paragraph or a list item over `LongBlockCap` characters, which is `1500`. A list item counts too. `fix` divides it into paragraphs of `LongBlockTarget` characters and `LongBlockWordTarget` words or less, with a blank line between them. The word bound keeps each part under a paragraph cap that counts words. A list item indents each later part to its content, which keeps the part in the item. A division lands at a sentence end, then at a sentence end inside a parenthesis, then between words. It never lands inside a code span, a link, bold text or a quotation, or before text that opens a block. The markdown gate admits the blank line, and refuses an edit that moves a paragraph to another container.

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
| Exemptions | function-word gap, a longer number, arithmetic, status code, label, unit before the noun, a measure that takes a singular verb | arithmetic, function-word gap, status code, label, unit before the noun, a measure that takes a singular verb | status code, exit status, literal, section sign, currency, quotation, word size, a measure that takes a singular verb, a number word in a hyphenated compound |

Prose requires a frame, because a document carries numbers that count nothing: a version, a port, an example. The gate needs no frame. That is the whole difference between the document substrates. A number beside code is nearly always a count. A comment therefore needs no frame either. The vocabularies stay separate, because widening one changes the verdict on text nobody edited. `Find` returns the whole quantity for prose. That `cardinal.Leading` can cut its number. For a comment it returns the number alone.

## counts: a count in a document

`counts/inventory-count` cuts the cardinal out of a sentence that counts what is here. `there are three sections` becomes `there are sections`, which stays true. The org rules that a count in markdown is not worth maintaining.

Every reported count gets a repair. Where a bare cut breaks the sentence, `counts/reword.go` writes words that state no figure. A rate becomes `every few`. A cap becomes `a bounded number of`. A unit takes `a couple of`, `a few`, `several` or `many`, by size. After a preposition or a noun the number becomes `multiple`. A hedge such as `about` or `exactly` goes with the number. A zero becomes `no`. A label goes stale like a count. It is a finding too, and `counts/label.go` names the item instead. Digits after a singular noun name an item, as in `branch 3 sees` (`cardinal.NamesAnItem`). The repair takes the words the text gives that item, as in `branch (3) the keyless fallback` or `Keyless fallback (branch 3)`. With none, it writes `one id` or `a later branch`. A plural noun after the label is what the label describes: `rule 6 inputs` becomes `the inputs of a later rule`. A point sits after `at`, `by` or `to`, before a noun that is not a unit (`cardinal.APoint`). In a comment it takes the name of a constant with that value, in backticks. Otherwise it takes `a set number of` before a noun, or `a set limit`, `a set value` or `a set amount`. A bare number that the same clause sets beside it, as in `warns at 500 lines and errors at 750`, says how it compares: `a higher count`, `a lower count` or `the same count`. No clause stays half converted.

A count needs a frame and a quantity on the same line. The quantity is a cardinal that governs a plural noun. The frame is a possessive (`this repo's plugins`), a having verb (`it ships hooks`) or a deictic (`the rules below`). A quantity with no frame is ordinary technical prose.

It does not flag the singular, which is overwhelmingly a pronoun in English. That gap is stated rather than closed. It drops a quantity reached through a function word, as in `2 of the format drops`. It drops a match that continues a longer number, such as a version. Fences, tables, headings and backtick spans never reach it. A measurement inside a frame IS reported, because the number belongs where it is enforced.

A number behind a section sign cites a section, so neither count rule reads it. `counts/section-number` repairs it instead. A section number goes stale when a section is inserted above it. The repair writes a link named by a slug of the heading's title: `§9` becomes `[§owning-renderer](#9-owning-the-renderer)`. The slug keeps the first `slugWords` words that name the section. A repeated slug gets `-2`, and the anchor follows GitHub. The prose rules read the link as the citation it replaced, which holds no word. A file named right before the citation, or after it with `of`, resolves beside the document. A citation that no numbered heading answers keeps its number.

## tombstones: a comment about a state the code has left

A tombstone describes a state the code has left, or argues for the diff instead of telling the next editor what breaks. The wording tier is the weakest. A paraphrasing machine writes the judged text. It eventually writes around a phrase rule.

- The wording tells are `pattern` entries with an `id` in `english/english.xml`. Each cuts the PHRASE, not the line. The rest of the sentence thus survives. A pattern with no `id` still rewrites.
- `tombstones/name-nothing-in-the-repository-defines` reads no wording. A comment that names a symbol found nowhere in the repository describes a tree that is gone. The probe runs ripgrep on the working tree. A tree walk, and any run past `50` files, first reads the identifiers of every file git lists. That list holds tracked files and untracked files git does not ignore. The index then answers every name with no probe.
- A pattern cut never removes a negation while the words it negates stay. `no longer` is therefore a flag, not a cut.
- A copyright year is cut on purpose. A yearly bump only pads a commit.
- `tombstones/comment-volume` counts the lines of a merged comment run against the cap, which defaults to `14` lines. No rewording defeats it. The repair cuts the run from its end down to the cap, the way `comments/length` cuts. With no sentence left to cut, it drops whole lines only when the last line it keeps ends a sentence. Otherwise the run stays for a rewrite by hand.
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
- `comments/tail`: a comment that stops on a word that opens what a cut took away. The repair closes the sentence where its last clause that stands alone ends.
- `ste/sentence-length`: a sentence over the cap in a comment's prose, in every language above and in a workflow. It is a check of its own beside `comments/length`, which weighs the comment against its code. Directives, license notices, verbatim paragraphs, code spans and quotations stay out. `comments/sentence-length` divides the sentence as `ste` does in a document. It writes the result back through the comment gate, with the comment's marker, indent and wrap. `sentencecap_test.go` proves that both repairs settle in one fix.

The length repair cuts from the end, because a comment leads with its point. Each cut lands on a sentence end. The opening sentence is never cut mid-clause. When no cut fits, the opening sentence stays, repaired to STE. One over the 25-word cap closes at a clause boundary the `syntax` parser finds, the same boundaries `ste/sentence-length` divides at (`ste.Leading`). Sentence ends come from `ste.Sentences`. A cut keeps text only when its last sentence stands alone (`ste.StandsAlone`). A cut may also land before a tail opener of `rules/comment-clauses.xml`, such as `so`, `because` or an open parenthesis. A cut before `when`, `if` or `that` changes the claim, so none lands there. When no clause boundary fits either, the comment stays as written, and the finding asks for a rewrite by hand. A cut between words leaves a fragment, so no repair makes one. The number repair runs after the length cut. The length cut then runs again if the words overflow.

It does not flag a comment above the package declaration, or a trailing comment on a code line. It skips a directive line, such as a build constraint, a shebang, a linter pragma or a cgo preamble. It skips a number inside a quotation.

## yaml: workflow and action manifest rules

[docs/yaml-workflow-and-action-manifest-rules.md](docs/yaml-workflow-and-action-manifest-rules.md) holds this section.

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

Denials: `heredoc`, `perl` (`^perl[0-9.]*$` as the effective command), `shred`, `git_rm` with `--pathspec-from-file`, `truncate_zero` with an unknown flag, `rm_flag` for an `rm` flag it cannot drop. And `toolchain_capture` for `go-toolchain` inside a substitution.

`cat`, `head`, `tail` and `sed` run as written. `readplan.go` maps one plain read onto Read calls, and `check read-plan` prints that mapping for one command.

- `devnull` removes `2>/dev/null` in any spelling. It matches a parsed redirect. A `12>/dev/null` thus stays.
- `rm_recycle`, `truncate_recycle` and `find_delete_recycle` turn deletion into `recycler trash`. Delete-then-Write is the loophole around the Write refusal.
- `git_rm_recycle` writes `git rm` as `git rm --cached` and then `recycler trash` on the same paths, inside each `-C` directory. A dry run passes through.
- `grep`, `or_true` and `stderr_merge` drop a trailing `| grep`, `|| true` or `2>&1` from the final statement.
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
