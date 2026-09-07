# ste

ASD-STE100 is Simplified Technical English, a controlled language. Each approved word carries a single meaning and a single part of speech, and its rules keep every sentence to a single reading. This package reports what a run of prose breaks, and repairs what a rewrite can repair.

## A family, and why it is not split further

The members share the sentence splitter, the word pattern, and the masks that hide an inline code span, a link target and an HTML entity. They also share the repair pass, which applies the word, semicolon and splice rewrites together over the same prose. A directory per member leaves that machinery in a package holding no rule of its own. The family therefore keeps one directory and one README.

Each member still selects on its own by ID. A caller wanting a semicolon repaired never adopts the others.

## The prose these rules read is substrate-independent

STE governs prose. A code comment, a commit message and a plain text file carry prose exactly as a document does. Markdown is not what these rules are about. It is a substrate the caller parses to find where the prose is.

The prose reaching `Check` is already a block joined to a single line. Finding that block is somebody else's job.

## The members

`ste/contraction` rejects a contraction. STE approves none of them. It repairs, writing the expansion and keeping the capitalization the source used.

`ste/modal` rejects `should`, `shall`, `could`, `might` and `would`. It repairs, writing the approved word for that sense. Obligation becomes `must`, and possibility becomes `can`.

`ste/semicolon` rejects the semicolon. It repairs, writing the period the semicolon stands in for and capitalizing the next word.

`ste/comma-splice` rejects a comma joining clauses that each stand alone. That is the semicolon spelled differently. It repairs the same way, and a conjunction after the comma survives to open the new sentence.

`ste/sentence-length` rejects a sentence over the word cap, which is the STE cap for a description. It reports only. Splitting a sentence needs a writer who knows which half is the point.

`ste/count` rejects a stated count of items. The number is true until somebody changes the set, and nothing corrects it then. It reports only.

## Worked example

```md
It should work; it does not. `a; b` stays.
```

`slopfix fix --only ste/semicolon` writes the period and leaves the modal alone, because no caller asked for it.

```md
It should work. It does not. `a; b` stays.
```

`slopfix check` on the same line reports the modal and the semicolon against the line each sits on.

## What it does not flag

An inline code span, a markdown link target and an HTML entity are masked before any rule reads the text. An entity ends in a semicolon, and unmasked prose reports that as its own. A repair leaves each span exactly as it was.

A comma splice needs a subject and a finite verb after the comma. That test leaves a list and an Oxford comma alone, because neither carries a verb. A participle and an infinitive do not mark a clause either. With no conjunction the words before the comma must carry a finite verb too. An introductory phrase is therefore left alone.

Text in parentheses counts as a single word toward the cap, which is what STE says. A citation therefore cannot inflate a sentence past the cap.

A period ends a sentence only when what follows opens the next. That rules out `e.g.` and the `$(...)` of a shell example. A plain split on a period cuts an oversized sentence into short pieces, and it then escapes the cap. A lower-case opener still starts a sentence when it is a file name or a section mark.

The stale-count member exempts a unit. A budget in characters names a size, and nobody revisits it when a list grows. It also exempts a number that is arithmetic rather than a count, such as a range or an expression.

## Running a member

```sh
slopfix check doc.md
slopfix fix --only ste doc.md
slopfix fix --only ste/semicolon --path doc.md < doc.md
slopfix report --path doc.md --only ste/modal < doc.md
```

Naming a member also joins the paragraph it repairs. A rule reads a paragraph as a sentence stream, and a hand wrap hides half of it.

## Which hook selects it

The `common-checks` plugin, at PreToolUse. It names no rule at all: this family is part of the default check set, which is what that gate means.
