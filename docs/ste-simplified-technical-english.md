# ste: Simplified Technical English

ASD-STE100 is a controlled language. Each approved word has a single meaning and part of speech. Its rules keep each sentence to a single reading. STE governs prose. It applies to a comment or a commit message as much as a document. The text that reaches `Check` is already a block joined to a single line.

The `ste` package also runs rules that are not STE: plain English and the counts policy. They share the sentence splitter, the masks and the repair pass, so they share the package. The ID prefix names the category, and each rule still selects on its own by ID.

- `ste/contraction`: a contraction. The repair writes the expansion and keeps the capitalization.
- `ste/modal`: `should`, `shall`, `could`, `might` and `would`. The repair writes `must` for obligation and `can` for possibility.
- `ste/sentence-length`: a sentence over `25` words, the STE cap for a description. The repair divides it at a clause boundary that the `syntax` parser finds. With no such boundary, it divides between words near the cap.
- `english/semicolon`: the semicolon. The repair writes a period and capitalizes the next word.
- `english/comma-splice`: a comma that joins clauses that each stand alone. The repair writes a period. A connector replaces the conjunction: `However,` for `but` and `As a result,` for `so`. It drops `and`.
- `counts/postdeterminer`: a numeral between a determiner and its noun, as in `the three rules`. The repair cuts the numeral. A unit, a percent, a year, a status code, `any` and `first` keep theirs.
- `counts/stated-count`: a stated count anywhere in the line, read with the `Gate` substrate. The repair takes the number out after the join, with the `counts` rewording.

The warning rules read patterns that need a person to repair. A warning never fails `check`. `check --json` gives each finding a `severity` of `error` or `warning`. The language server sends a warning at warning level.

- `ste/instruction-length`: a sentence of `21` to `25` words, over the STE cap for an instruction.
- `ste/passive`: a form of `be` and a past participle, as the parser tags them.
- `ste/noun-cluster`: a run of nouns longer than `NounClusterCap`, which is `3`.
- `ste/tense`: a perfect or a progressive tense. The approved `-ing` words, such as `missing` and `during`, do not count.
- `ste/dictionary`: a word the STE dictionary does not approve, with its approved replacements. The table is the `dictionary` list in `rules/ste-dictionary.xml`. A `<list>` holds a string on each line of its text and needs no `<test>`. It holds only words with no approved sense, because a match by spelling cannot tell senses apart.
- `ste/paragraph-length`: a paragraph with more sentences than `ParagraphSentenceCap`, which is `6`. A list item never counts.

How a long sentence divides:

- A clause after `and`, `but` or `so` that names its own subject starts the new sentence.
- A verb group that shares the subject gets it again. A long subject becomes an agreeing pronoun.
- A closing `, which` clause opens with `This`. A closing `because` clause opens with `This is because`.
- A list, a quotation and a subordinate clause never divide at a clause boundary.
- A sentence with no boundary divides between words, as close to the cap as a good place allows. A comma. Then a word that opens a phrase, rank first. The first part never ends on a word that opens what it leaves out.
- The forced division never lands inside a code span, a link, a quotation, a parenthesis or bold text. The rest opens as it stands when it names its own subject. A verb gets the subject again, `which` becomes `This`, and anything else opens with `This is`.

What the package does not flag:

- An inline code span, a link target and an HTML entity are masked. Every repair leaves them as they are.
- A comma splice needs a subject and a finite verb after the comma. A list, an Oxford comma, a participle and an infinitive do not qualify. Without a conjunction, the words before the comma must be a main clause.
- No repair rewrites inside a quotation.
- Text in parentheses counts as a single word. A citation thus cannot inflate a sentence.
- A period ends a sentence only when what follows opens the next. That rules out `e.g.` and `$(...)`. A file name or a section mark can open a sentence in lower case.
- `counts/stated-count` exempts arithmetic, such as a range or an expression. It exempts no noun, because a duration or a size goes stale too.
- The literal shape `issue #<digits>` is not a count. A number that picks an item from a list, as in `gate 5`, is a count.
- A cut keeps the sentence English. `over` after a noun stays as a preposition. A capital moves only at the start of a sentence.
