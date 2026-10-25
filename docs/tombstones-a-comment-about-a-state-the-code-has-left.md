# tombstones: a comment about a state the code has left

A tombstone describes a state the code left, or argues for the diff instead of telling the next editor what breaks. The wording tier is the weakest. A paraphrasing machine writes the judged text. It eventually writes around a phrase rule.

- The wording tells are `pattern` entries with an `id` in `english/english.xml`. Each cuts the PHRASE, not the line. The rest of the sentence thus survives. A pattern with no `id` still rewrites.
- `tombstones/name-nothing-in-the-repository-defines` reads no wording. A comment that names a symbol found nowhere in the repository describes a tree that is gone. The probe runs ripgrep on the working tree. A tree walk, and any run past `50` files, first reads the identifiers of every file git lists. That list holds tracked files and untracked files git does not ignore. It also holds the files on disk under a submodule path that holds a copy in place of a checkout. The index then answers every name with no probe.
- A pattern cut never removes a negation while the words it negates stay. `no longer` is therefore a flag, not a cut.
- A copyright year is cut on purpose. A yearly bump only pads a commit.
- `tombstones/comment-volume` counts the lines of a merged comment run against the cap, which defaults to `14` lines. No rewording defeats it. The repair cuts the run from its end down to the cap, the way `comments/length` cuts. With no sentence left to cut, it drops whole lines only when the last line it keeps ends a sentence. Otherwise the run stays for a rewrite by hand.
- A dead name that no whole-line strip resolves loses the sentence that holds it. On a line shared with code, the cut stops at the comment marker.

A referent candidate must look like a symbol: an underscore, an internal case change, or a capital beside a digit. A name in capitals and a short name never qualify. The probe answers nothing when it cannot answer: no repository, no ripgrep, a timeout or an error. An absent answer must never read as a missing symbol. A comment that names too many candidates is skipped. A file that cites a line of a C family source the repository does not hold, such as `UnrealNames.cpp:4588`, ports that codebase.

Its comments and those of its test partner name a foreign API, so the probe answers nothing for them.

Only prose is judged. In source that means the comments. A string that holds `previously` is thus safe. In a document it means the paragraphs, without fences, HTML comments, frontmatter or backtick spans. A file with no known comment syntax is not judged. A document has no volume cap.

A referent line strips only when the comment is alone on its source line. A document finding never strips, because a document line is a paragraph. The block is rewrapped after a strip. The response names each deleted line. Git history keeps the rest.

`--path` is required on stdin, because the path decides the comment syntax.

```sh
slopfix fix --only tombstones --max-comment-lines 0 --path pkg/thing.go - < pkg/thing.go
```
