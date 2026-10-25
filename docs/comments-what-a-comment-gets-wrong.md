# comments: what a comment gets wrong

The `commentfix` package reads a real syntax tree through `go-tree-sitter`. A comment is a node whose type carries `comment`. The documented construct is the next named sibling. No rule names a language. A file that does not parse yields nothing, because half a tree is worse than none.

The languages are Go, C, C++, Rust, Bash, JavaScript, TypeScript and TSX. YAML. TOML, `.conf` and `.zsh` files read with the Bash grammar. `languages_test.go` refuses a grammar that no fixture proves. `selfrepair_test.go` runs the rules over this tree. The go-toolchain vet phase runs them on every build.

- `comments/number`: a number in a comment, read with the `Comment` substrate. The repair says it in words. A count of a plural noun that no table entry covers gets the `counts` rewording. Only a sentence that still holds a number after that is cut. To point at a section, cite its slug or heading, not its position.
- `comments/length`: a comment run weighed against the construct beneath it. Lines catch an essay. Characters catch a dense paragraph. The budget has a floor. A short comment is never a finding.
- `comments/tail`: a comment that stops on a word that opens what a cut took away. The repair closes the sentence where its last clause that stands alone ends.
- `comments/header`, a warning: a source file that opens on code, past a shebang, directives and a license block. It reports only a file with a comment on a top-level declaration whose name the tree gives. A test file and a generated file stay out. The repair writes `FILE holds NAME.` at the top as a line comment, with a blank line under it. A copy of the comment will say the same thing twice. A Go header never becomes the package doc. `commentfix.FixTree` writes the header in each file it writes, and lists every other file of the walk with no header in `TreeResult.Headers`.
- `ste/sentence-length`: a sentence over the cap in a comment's prose, in every language above and in a workflow. It is a check of its own beside `comments/length`, which weighs the comment against its code. Directives, license notices, verbatim paragraphs, code spans and quotations stay out. `comments/sentence-length` divides the sentence as `ste` does in a document. It writes the result back through the comment gate, with the comment's marker, indent and wrap. `sentencecap_test.go` proves that both repairs settle in one fix.

The length repair cuts from the end, because a comment leads with its point. Each cut lands on a sentence end. The opening sentence is never cut mid-clause. When no cut fits, the opening sentence stays, repaired to STE. One over the 25-word cap closes at a clause boundary the `syntax` parser finds, the same boundaries `ste/sentence-length` divides at (`ste.Leading`). Sentence ends come from `ste.Sentences`.

A cut keeps text only when its last sentence stands alone (`ste.StandsAlone`). A cut may also land before a tail opener of `rules/comment-clauses.xml`, such as `so`, `because` or an open parenthesis. A cut before `when`, `if` or `that` changes the claim, so none lands there. When no clause boundary fits either, the comment stays as written, and the finding asks for a rewrite by hand. A cut between words leaves a fragment, so no repair makes one. The number repair runs after the length cut.

The length cut then runs again if the words overflow.

It does not flag a comment above the package declaration, or a trailing comment on a code line. It skips a directive line, such as a build constraint, a shebang, a linter pragma, a TypeScript triple-slash directive or a cgo preamble. It skips a number inside a quotation.
