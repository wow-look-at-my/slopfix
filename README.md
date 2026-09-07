# slopfmt

One tool for the prose rules this org applies to a markdown file, and for the same rules a code comment must follow. It also reads a GitHub Actions workflow and an action manifest, where the org's gate rejects other things.

## Build

```sh
go-toolchain            # builds build/slopfmt, and runs the tests
```

## Use

```sh
slopfmt check docs/*.md   # report what the rules reject, and exit 1 when anything does
slopfmt fix docs/*.md     # repair each file in place, and report what is left
slopfmt fmt docs/*.md     # join every wrapped paragraph back to a single line
slopfmt purge .           # delete the markdown a repository must not keep
slopfmt comments .        # report a number stated in a comment, in any language
slopfmt workflows .       # read every workflow and action manifest in the tree
```

`fix` also reads a document on stdin and writes the repaired one on stdout. With `--json` the whole answer is one object, which is what a hook reads.

`comments` reads source rather than prose. A number in a comment is a count of what exists today, and the edit that adds an item leaves it wrong. It reads a comment by its delimiters rather than by a grammar. So it answers for every language it knows, and on a tree that does not compile. A directory is walked, skipping hidden directories, `vendor`, `node_modules`, `testdata` and `build`. A named file is read whatever its extension. go-toolchain runs this same check as its first phase.

## Naming the rules to run

`--only` takes a comma-separated list. An entry is a category, or a rule ID inside a category. A rule ID is the name the report prints next to the finding, the way a compiler names a warning. What a message says and what a caller asks for are the same word.

```sh
slopfmt fix --only counts            # every rule in the counts category
slopfmt fix --only ste/semicolon     # that rule alone, and no other in ste
slopfmt fix --only tombstones,wrap   # two categories
slopfmt fix --only ste/nosuch        # an error naming the rules ste holds
```

The categories are `tombstones`, `counts`, `wrap` and `ste`. A rule ID turns its category on, so naming a rule never needs the category named beside it. An unknown name is an error rather than a silent no-op, because a run that applies nothing reads as a clean file.

A word repair and the wrap join share a pass. A rule reads a paragraph as a sentence stream, and a hand wrap hides half of it. So naming a `ste` rule also joins the paragraph it repairs.

## What fix repairs

- A wrapped paragraph, joined back to a single line.
- A contraction, written out. A banned modal, replaced by the approved word.
- A semicolon and a comma splice, each replaced by the period it stands in for.
- The cardinal in a stated count, cut out. The sentence then stays true.

A sentence over the word cap is reported and left alone. To split one, the writer must know which half is the point.

## What check reports

- A paragraph split over several lines. The reader's window wraps a paragraph. An author's wrap freezes one window's width into the file.
- A semicolon, a contraction, and a banned modal. STE approves none of them.
- A comma joining clauses that each stand alone. The rule wants a subject and a finite verb after the comma.
- A sentence over the word cap. Text in parentheses counts as a single word, which keeps a citation from inflating the count.
- A stated count of items. The number is true until somebody changes the set, and nothing corrects it then.
- A tombstone comment. It describes a state the code has left, or argues for the diff instead of telling the next editor what breaks.

## What check reports in a workflow

A workflow under `.github/workflows`, and an `action.yml` beside it, are read by rules of their own. The prose rules never run on either. `fmt` refuses one outright, because a newline there is syntax. Joining a two-line `concurrency:` block makes GitHub reject the file before a job starts.

`workflows` walks a tree for them, and `--exclude` takes a glob for a fixture that breaks a rule on purpose. The walk keeps `.github`, which the other walks skip as a hidden directory. A walk that selects no file exits non-zero. A run that read nothing enforced nothing, and in CI that means the step ran ahead of the checkout.

- A run of comment lines. The limit is one line. Say what a reader needs right there, and put the rest in the commit message.
- A job named `all-builds`, by its key or by its name. The required gate is a commit status from the required-builds-manager app. A job wearing the name satisfies nothing, and shadows the real gate in the UI.
- A test written into a `run:` script. That covers an assertion, a shell function whose name says it asserts, or a redirect naming a test file. A step that merely runs a command fails on its own exit code, and is left alone.

## What no rule reads

A fenced code block, a table and a heading are data. No rule reads them. The `fmt` command never reflows them.

An HTML entity ends in a semicolon, and an inline code span holds whatever it holds. Both are masked before a rule sees the text, and `fix` leaves each of them as it is.

`fmt` carries a guarantee `fix` keeps: it moves newlines and nothing else. A rewrite whose words differ from the source is refused rather than written.
